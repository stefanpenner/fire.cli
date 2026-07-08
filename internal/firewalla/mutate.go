package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Mutations go through Firewalla's own PolicyManager2 (driven via node in the
// firewalla app dir) so the change is both persisted AND enforced by the
// running FireMain process — exactly as the app does it. Writing redis directly
// would persist without enforcing.

// nodePreamble locates the box's bundled node and enters the app directory.
const nodePreamble = `NODE=$(ls /home/pi/.nvm/versions/node/*/bin/node 2>/dev/null | head -1); ` +
	`[ -z "$NODE" ] && NODE=$(command -v node); cd /home/pi/firewalla && `

// pm2Script wraps a body with PolicyManager2 + initialized SysManager. The body
// runs as the async function with `pm2`, `Policy`, and `process.env` in scope,
// and must console.log its JSON result and process.exit.
func pm2Script(body string) string {
	return `const sysManager=require("./net2/SysManager.js");` +
		`const Policy=require("./alarm/Policy.js");` +
		`const PM2=require("./alarm/PolicyManager2.js");const pm2=new PM2();` +
		`(async()=>{await sysManager.updateAsync();` + body +
		`})().catch(e=>{console.error(e&&(e.message||e));process.exit(1);});`
}

// runNode executes a node script body on the box with the given env vars.
func (c *Client) runNode(ctx context.Context, body string, env map[string]string) (string, error) {
	var prefix strings.Builder
	for k, v := range env {
		fmt.Fprintf(&prefix, "%s=%s ", k, shellQuote(v))
	}
	cmd := nodePreamble + prefix.String() + `"$NODE" -e ` + shellQuote(body)
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return res.Stdout, fmt.Errorf("policy operation on %s failed: %s: %w", c.t.Host(), msg, err)
	}
	return res.Stdout, nil
}

// RuleSpec describes a rule to create (block/allow a target).
type RuleSpec struct {
	Action    string `json:"action"`              // block | allow
	Type      string `json:"type"`                // mac | ip | dns | category | net | …
	Target    string `json:"target"`              // the matched value
	Direction string `json:"direction,omitempty"` // default bidirection
	Notes     string `json:"notes,omitempty"`
	ExpireSec int    `json:"expire,omitempty"` // 0 = permanent
}

// CreateRule creates and enforces a rule, returning the new policy id.
func (c *Client) CreateRule(ctx context.Context, spec RuleSpec) (string, error) {
	if spec.Direction == "" {
		spec.Direction = "bidirection"
	}
	payload, _ := json.Marshal(spec)
	body := `const v=JSON.parse(process.env.FIRE_POLICY);` +
		`const {policy,alreadyExists}=await pm2.checkAndSaveAsync(new Policy(v));` +
		`console.log(JSON.stringify({pid:policy.pid,exists:alreadyExists||"no"}));process.exit(0);`
	out, err := c.runNode(ctx, pm2Script(body), map[string]string{"FIRE_POLICY": string(payload)})
	if err != nil {
		return "", err
	}
	var res struct {
		PID string `json:"pid"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(lastLine(out))), &res)
	return res.PID, nil
}

// DeleteMatching removes every rule matching spec (used to undo a block).
// Returns the number of rules removed.
func (c *Client) DeleteMatching(ctx context.Context, spec RuleSpec) (int, error) {
	if spec.Direction == "" {
		spec.Direction = "bidirection"
	}
	payload, _ := json.Marshal(spec)
	body := `const v=JSON.parse(process.env.FIRE_POLICY);` +
		`const sames=await pm2.getSamePolicies(new Policy(v));let n=0;` +
		`for(const p of (sames||[])){await pm2.disableAndDeletePolicy(p.pid);n++;}` +
		`console.log(JSON.stringify({removed:n}));process.exit(0);`
	out, err := c.runNode(ctx, pm2Script(body), map[string]string{"FIRE_POLICY": string(payload)})
	if err != nil {
		return 0, err
	}
	var res struct {
		Removed int `json:"removed"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(lastLine(out))), &res)
	return res.Removed, nil
}

// DeleteRule removes a rule by policy id.
func (c *Client) DeleteRule(ctx context.Context, id string) error {
	body := `const id=process.env.FIRE_PID;const p=await pm2.getPolicy(id);` +
		`if(!p){console.error("no rule "+id);process.exit(2);}` +
		`await pm2.disableAndDeletePolicy(id);console.log("ok");process.exit(0);`
	_, err := c.runNode(ctx, pm2Script(body), map[string]string{"FIRE_PID": id})
	return err
}

// SetRuleDisabled enables or disables a rule by policy id.
func (c *Client) SetRuleDisabled(ctx context.Context, id string, disabled bool) error {
	body := `const id=process.env.FIRE_PID;const p=await pm2.getPolicy(id);` +
		`if(!p){console.error("no rule "+id);process.exit(2);}` +
		`if(process.env.FIRE_DISABLE==="1"){await pm2.disablePolicy(p);}else{await pm2.enablePolicy(p);}` +
		`console.log("ok");process.exit(0);`
	env := map[string]string{"FIRE_PID": id, "FIRE_DISABLE": "0"}
	if disabled {
		env["FIRE_DISABLE"] = "1"
	}
	_, err := c.runNode(ctx, pm2Script(body), env)
	return err
}

// SetFeature enables or disables a box-wide feature (a policy:system key such
// as adblock, family, doh) the way the app does: through HostManager so the
// change is persisted AND the running FireMain enforces it. Object-valued
// features (e.g. {state, …}) keep their other fields and only flip "state".
func (c *Client) SetFeature(ctx context.Context, key string, enabled bool) error {
	body := `const HostManager=require("./net2/HostManager.js");const hm=new HostManager();` +
		`const key=process.env.FIRE_KEY;const on=process.env.FIRE_ON==="1";` +
		`await hm.loadPolicyAsync();` +
		`const cur=hm.policy?hm.policy[key]:undefined;let val=on;` +
		`if(cur&&typeof cur==="object"&&!Array.isArray(cur)){val=Object.assign({},cur,{state:on});}` +
		`await hm.setPolicyAsync(key,val);` +
		`console.log(JSON.stringify({key:key,state:on}));process.exit(0);`
	env := map[string]string{"FIRE_KEY": key, "FIRE_ON": "0"}
	if enabled {
		env["FIRE_ON"] = "1"
	}
	// HostManager scripts don't need PolicyManager2; run the body directly.
	_, err := c.runNode(ctx, wrapAsync(body), env)
	return err
}

// ArchiveAlarm acknowledges an alarm by moving it from the active queue to the
// archive (the app's "ignore"/dismiss), via AlarmManager2 so the running
// process stays consistent.
func (c *Client) ArchiveAlarm(ctx context.Context, id string) error {
	return c.alarmOp(ctx, id, "archive")
}

// DeleteAlarm removes an alarm entirely.
func (c *Client) DeleteAlarm(ctx context.Context, id string) error {
	return c.alarmOp(ctx, id, "delete")
}

func (c *Client) alarmOp(ctx context.Context, id, op string) error {
	body := `const AM2=require("./alarm/AlarmManager2.js");const am2=new AM2();` +
		`const id=process.env.FIRE_AID;` +
		`if(process.env.FIRE_OP==="delete"){await am2.removeAlarmAsync(id);}` +
		`else{await am2.archiveAlarm(id);}` +
		`console.log("ok");process.exit(0);`
	_, err := c.runNode(ctx, wrapAsync(body), map[string]string{"FIRE_AID": id, "FIRE_OP": op})
	return err
}

// wrapAsync wraps a body as an immediately-invoked async function with a
// uniform error handler. The body must console.log its result and process.exit.
func wrapAsync(body string) string {
	return `(async()=>{` + body +
		`})().catch(e=>{console.error(e&&(e.message||e));process.exit(1);});`
}

// lastLine returns the last non-empty line of s (node logging may precede our
// JSON result line).
func lastLine(s string) string {
	lines := splitLines(s)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}
