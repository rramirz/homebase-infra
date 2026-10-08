#!/usr/bin/env bash
# Static render contract tests for charts/paperclip.
#
# Deterministic, local-only: never contacts the cluster or the network.
# Requires only `helm` and `ruby` (macOS system ruby stdlib yaml) — no new
# dependencies. Run from the chart directory (via `task paperclip:test`).
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$CHART_DIR"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass() { echo "ok   - $1"; }
fail() { echo "FAIL - $1" >&2; exit 1; }

# render <release> <namespace> <out> <err> [helm flags...]
render() {
  local release="$1" ns="$2" out="$3" err="$4"; shift 4
  helm template "$release" . -n "$ns" "$@" > "$out" 2> "$err"
}

zero_resources() {
  ! grep -qE '^kind:' "$1" || fail "$2 produced resources but should render zero"
}

# Throwaway NON-REAL secrets (temp files only, never committed).
cat > "$TMP/strict-secrets.yaml" <<'SECRETS'
secrets:
  postgres:
    password: test-only-postgres-password-000000000000000000000000000000000000000000
  betterAuthSecret: test-only-better-auth-secret-1111111111111111111111111111111111111111
  toolActionSigningSecret: test-only-tool-signing-secret-222222222222222222222222222222222222
  agentJwtSecret: test-only-agent-jwt-secret-33333333333333333333333333333333333333333333
SECRETS

cat > "$TMP/space-secrets.yaml" <<'SECRETS'
secrets:
  postgres:
    password: "test only password with space and hash#char 000000000000000000000000000"
  betterAuthSecret: test-only-better-auth-secret-1111111111111111111111111111111111111111
  toolActionSigningSecret: test-only-tool-signing-secret-222222222222222222222222222222222222
  agentJwtSecret: test-only-agent-jwt-secret-33333333333333333333333333333333333333333333
SECRETS

# --- 1. base (enabled: false) -------------------------------------------------
render paperclip-base default "$TMP/base.yaml" "$TMP/base.err" -f values.yaml \
  || { cat "$TMP/base.err" >&2; fail "base render should succeed (disabled)"; }
zero_resources "$TMP/base.yaml" "base (enabled:false)"
pass "base (enabled:false) renders zero resources"

# --- 2. dormant work overlay --------------------------------------------------
render paperclip-work paperclip-work "$TMP/work.yaml" "$TMP/work.err" -f values.yaml -f values-work.yaml \
  || { cat "$TMP/work.err" >&2; fail "work overlay render should succeed (disabled)"; }
zero_resources "$TMP/work.yaml" "work overlay (disabled)"
pass "work overlay (disabled) renders zero resources"

# --- 3. dormant logicbroker overlay ------------------------------------------
render paperclip-logicbroker paperclip-logicbroker "$TMP/lb.yaml" "$TMP/lb.err" -f values.yaml -f values-logicbroker.yaml \
  || { cat "$TMP/lb.err" >&2; fail "logicbroker overlay render should succeed (disabled)"; }
zero_resources "$TMP/lb.yaml" "logicbroker overlay (disabled)"
pass "logicbroker overlay (disabled) renders zero resources"

# --- 4. personal (bootstrap) WITHOUT secrets must fail -----------------------
if render paperclip-personal paperclip-personal "$TMP/nosec.yaml" "$TMP/nosec.err" -f values.yaml -f values-personal.yaml; then
  fail "personal render without secrets should have failed (empty secret)"
fi
pass "personal render without secrets fails"

# --- 5. strict guards (claimed stack + real secrets, testMode=false) ---------
# 5a. wrong release name
if render paperclip-wrong paperclip-personal "$TMP/a.yaml" "$TMP/a.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml"; then
  fail "wrong release name should have failed"
fi
pass "wrong release name fails"

# 5b. wrong namespace
if render paperclip-personal paperclip-work "$TMP/b.yaml" "$TMP/b.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml"; then
  fail "wrong namespace should have failed"
fi
pass "wrong namespace fails"

# 5c. wrong public URL (claimed expects https://hostname)
if render paperclip-personal paperclip-personal "$TMP/c.yaml" "$TMP/c.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set-string env.PAPERCLIP_PUBLIC_URL=https://wrong.example.com; then
  fail "wrong PAPERCLIP_PUBLIC_URL should have failed"
fi
pass "wrong PAPERCLIP_PUBLIC_URL fails (claimed)"

# 5d. non-pinned image digest
if render paperclip-personal paperclip-personal "$TMP/d.yaml" "$TMP/d.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set image.paperclip.digest=latest; then
  fail "non-pinned image digest should have failed"
fi
pass "non-pinned image digest fails"

# 5e. malformed digests (short hex, non-hex)
if render paperclip-personal paperclip-personal "$TMP/e1.yaml" "$TMP/e1.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set image.paperclip.digest=sha256:1234; then
  fail "short sha256 digest should have failed"
fi
if render paperclip-personal paperclip-personal "$TMP/e2.yaml" "$TMP/e2.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set image.postgres.digest=sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz; then
  fail "non-hex sha256 digest should have failed"
fi
pass "malformed image digests fail (short + non-hex)"

# 5f. short secret (<32 chars)
if render paperclip-personal paperclip-personal "$TMP/f.yaml" "$TMP/f.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set-string secrets.postgres.password=short; then
  fail "short postgres password should have failed"
fi
pass "short (<32 char) secret fails"

# 5g. non-distinct secrets
if render paperclip-personal paperclip-personal "$TMP/g.yaml" "$TMP/g.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/strict-secrets.yaml" --set-string secrets.toolActionSigningSecret=test-only-better-auth-secret-1111111111111111111111111111111111111111; then
  fail "non-distinct secrets should have failed"
fi
pass "non-distinct auth/tool/JWT secrets fail"

# 5h. DATABASE_URL encoding (space -> %20, # -> %23, no +)
render paperclip-personal paperclip-personal "$TMP/enc.yaml" "$TMP/enc.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/space-secrets.yaml" \
  || { cat "$TMP/enc.err" >&2; fail "space-password render should succeed"; }
ruby - "$TMP/enc.yaml" <<'RUBY'
require 'yaml'
docs = YAML.load_stream(File.read(ARGV.fetch(0))).compact
secret = docs.find { |d| d['kind'] == 'Secret' }
url = secret['stringData']['DATABASE_URL']
raise 'space not %20-encoded' unless url.include?('%20')
raise 'hash not %23-encoded' unless url.include?('%23')
raise 'space leaked as +' if url.include?('+')
puts 'ok   - DATABASE_URL encodes spaces as %20 and reserved chars correctly'
RUBY

# --- 6. testMode must NOT bypass namespace/publicURL/image/secret checks ------
# 6a. wrong namespace under testMode
if render paperclip-personal paperclip-wrong "$TMP/tm-ns.yaml" "$TMP/tm-ns.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml; then
  fail "testMode + wrong namespace should have failed"
fi
pass "testMode cannot bypass namespace validation"

# 6b. wrong public URL under testMode
if render paperclip-personal paperclip-personal "$TMP/tm-url.yaml" "$TMP/tm-url.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml --set-string env.PAPERCLIP_PUBLIC_URL=https://wrong.example.com; then
  fail "testMode + wrong public URL should have failed"
fi
pass "testMode cannot bypass public-URL validation"

# 6c. non-pinned digest under testMode
if render paperclip-personal paperclip-personal "$TMP/tm-img.yaml" "$TMP/tm-img.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml --set image.paperclip.digest=latest; then
  fail "testMode + non-pinned digest should have failed"
fi
pass "testMode cannot bypass image-pin validation"

# 6d. empty secret under testMode
cat > "$TMP/tm-empty.yaml" <<'SECRETS'
testMode: true
secrets:
  postgres:
    password: ""
  betterAuthSecret: ""
  toolActionSigningSecret: ""
  agentJwtSecret: ""
SECRETS
if render paperclip-personal paperclip-personal "$TMP/tm-empty-o.yaml" "$TMP/tm-empty-e.yaml" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f "$TMP/tm-empty.yaml"; then
  fail "testMode + empty secret should have failed"
fi
pass "testMode cannot bypass empty-secret validation"

# 6e. arbitrary nonempty (non-static-validation) secret under testMode
if render paperclip-personal paperclip-personal "$TMP/tm-arb.yaml" "$TMP/tm-arb.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml --set-string secrets.betterAuthSecret=some-arbitrary-non-placeholder-value-12345678901234; then
  fail "testMode + arbitrary nonempty secret should have failed"
fi
pass "testMode rejects arbitrary nonempty (non-static-validation) secret"

# 6f. bootstrap + wrong probe Host header under testMode
if render paperclip-personal paperclip-personal "$TMP/tm-probe-b.yaml" "$TMP/tm-probe-b.err" -f values.yaml -f values-personal.yaml -f secret-values.example.yaml --set-string probe.hostHeader=paperclip.theramirez.casa; then
  fail "testMode + wrong bootstrap probe.hostHeader should have failed"
fi
pass "testMode cannot bypass bootstrap probe.hostHeader validation"

# 6g. claimed + wrong probe Host header under testMode
if render paperclip-personal paperclip-personal "$TMP/tm-probe-c.yaml" "$TMP/tm-probe-c.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml --set-string probe.hostHeader=localhost:3100; then
  fail "testMode + wrong claimed probe.hostHeader should have failed"
fi
pass "testMode cannot bypass claimed probe.hostHeader validation"

# --- 7. bootstrap render (testMode) — no HTTPRoute, localhost, signup open ---
render paperclip-personal paperclip-personal "$TMP/bootstrap.yaml" "$TMP/bootstrap.err" -f values.yaml -f values-personal.yaml -f secret-values.example.yaml \
  || { cat "$TMP/bootstrap.err" >&2; fail "bootstrap render with example secrets failed"; }
ruby - "$TMP/bootstrap.yaml" <<'RUBY'
require 'yaml'
docs = YAML.load_stream(File.read(ARGV.fetch(0))).compact
by_kind = Hash.new { |h, k| h[k] = [] }
docs.each { |d| by_kind[d['kind']] << d }

def assert(cond, msg)
  raise "ASSERT FAILED: #{msg}" unless cond
end

assert(!by_kind.key?('HTTPRoute'), 'bootstrap must have no HTTPRoute')
assert(by_kind.key?('Deployment') && by_kind.key?('StatefulSet'), 'bootstrap must have Deployment + StatefulSet')

deploy = by_kind['Deployment'].first
env = deploy.dig('spec', 'template', 'spec', 'containers', 0, 'env').each_with_object({}) { |e, h| h[e['name']] = e }
assert(env['PAPERCLIP_PUBLIC_URL'].dig('value') == 'http://localhost:3100', 'bootstrap public URL localhost')
assert(env['PAPERCLIP_AUTH_DISABLE_SIGN_UP'].dig('value') == 'false', 'bootstrap signup open (DISABLE_SIGN_UP=false)')
assert(env['PAPERCLIP_AUTH_RATE_LIMIT_ENABLED'].dig('value') == 'true', 'rate limit enabled')
assert(env['PAPERCLIP_AGENT_JWT_DISABLE_LEGACY_FALLBACK'].dig('value') == 'true', 'legacy fallback disabled')
assert(env['PAPERCLIP_AGENT_JWT_SECRET'].dig('valueFrom', 'secretKeyRef', 'key') == 'PAPERCLIP_AGENT_JWT_SECRET', 'agent JWT secret ref')

app = deploy.dig('spec', 'template', 'spec', 'containers', 0)
%w[startupProbe readinessProbe livenessProbe].each do |probe|
  assert(app.dig(probe, 'httpGet', 'httpHeaders', 0, 'name') == 'Host', "#{probe} has Host header")
  assert(app.dig(probe, 'httpGet', 'httpHeaders', 0, 'value') == 'localhost:3100', "#{probe} Host header = localhost:3100")
end

secret = by_kind['Secret'].first
assert(secret['stringData'].key?('PAPERCLIP_AGENT_JWT_SECRET'), 'secret has agent JWT key')

ns = by_kind['Namespace'].first
%w[enforce audit warn].each do |m|
  assert(ns.dig('metadata', 'labels', "pod-security.kubernetes.io/#{m}") == 'baseline', "PSA #{m}=baseline")
end

puts 'ok   - bootstrap render assertions passed (no route, localhost, signup open)'
RUBY

# --- 8. claimed render (testMode) — HTTPRoute, https, signup closed ----------
render paperclip-personal paperclip-personal "$TMP/claimed.yaml" "$TMP/claimed.err" -f values.yaml -f values-personal.yaml -f values-personal-claimed.yaml -f secret-values.example.yaml \
  || { cat "$TMP/claimed.err" >&2; fail "claimed render with example secrets failed"; }
ruby - "$TMP/claimed.yaml" <<'RUBY'
require 'yaml'
file = ARGV.fetch(0)
docs = YAML.load_stream(File.read(file)).compact
by_kind = Hash.new { |h, k| h[k] = [] }
docs.each { |d| by_kind[d['kind']] << d }

def assert(cond, msg)
  raise "ASSERT FAILED: #{msg}" unless cond
end

%w[Namespace Secret Service StatefulSet PersistentVolumeClaim Deployment HTTPRoute].each do |k|
  assert(by_kind.key?(k), "missing #{k}")
end
assert(by_kind['Service'].size == 2, 'expected 2 Services (app + postgres headless)')

ns = by_kind['Namespace'].first
assert(ns.dig('metadata', 'name') == 'paperclip-personal', 'namespace name')
assert(ns.dig('metadata', 'annotations', 'helm.sh/resource-policy') == 'keep', 'namespace keep annotation')
%w[enforce audit warn].each do |m|
  assert(ns.dig('metadata', 'labels', "pod-security.kubernetes.io/#{m}") == 'baseline', "PSA #{m}=baseline")
end

docs.each do |d|
  next if d['kind'] == 'Namespace'
  assert(d.dig('metadata', 'namespace') == 'paperclip-personal', "#{d['kind']} lives in paperclip-personal")
end

secret = by_kind['Secret'].first
sd = secret['stringData']
%w[POSTGRES_PASSWORD DATABASE_URL BETTER_AUTH_SECRET PAPERCLIP_TOOL_ACTION_SIGNING_SECRET PAPERCLIP_AGENT_JWT_SECRET].each do |k|
  assert(sd.key?(k), "secret key #{k}")
end
assert(sd['DATABASE_URL'].start_with?('postgresql://paperclip:'), 'DATABASE_URL scheme/user')

sts = by_kind['StatefulSet'].first
assert(sts.dig('metadata', 'name') == 'paperclip-personal-postgres', 'sts name')
pg = sts.dig('spec', 'template', 'spec', 'containers').first
assert(pg['image'] == 'docker.io/library/postgres:17-alpine@sha256:aa90e97ee862e558111d34cfb8b2c4bec768c2b039fb791341686928560263b3', 'postgres image pinned')
assert(pg.dig('securityContext', 'allowPrivilegeEscalation') == false, 'postgres allowPrivilegeEscalation false')
assert(!pg.dig('securityContext').key?('capabilities'), 'postgres must not drop capabilities')
vct = sts.dig('spec', 'volumeClaimTemplates', 0)
assert(vct.dig('spec', 'resources', 'requests', 'storage') == '10Gi', 'postgres storage 10Gi')
assert(vct.dig('spec', 'storageClassName') == 'ssd', 'postgres storageClass ssd')
assert(vct.dig('spec', 'accessModes') == ['ReadWriteOnce'], 'postgres RWO')

pvc = by_kind['PersistentVolumeClaim'].first
assert(pvc.dig('metadata', 'name') == 'paperclip-personal-paperclip', 'app pvc name')
assert(pvc.dig('metadata', 'annotations', 'helm.sh/resource-policy') == 'keep', 'app pvc keep annotation')
assert(pvc.dig('spec', 'resources', 'requests', 'storage') == '10Gi', 'app pvc 10Gi')
assert(pvc.dig('spec', 'storageClassName') == 'ssd', 'app pvc ssd')
assert(pvc.dig('spec', 'accessModes') == ['ReadWriteOnce'], 'app pvc RWO')

app_svc = by_kind['Service'].find { |s| s.dig('metadata', 'name') == 'paperclip-personal' }
assert(app_svc, 'app Service exists')
assert(app_svc.dig('spec', 'type') == 'ClusterIP', 'app Service ClusterIP')
assert(app_svc.dig('spec', 'ports', 0, 'port') == 3100, 'app Service port 3100')

deploy = by_kind['Deployment'].first
assert(deploy.dig('metadata', 'name') == 'paperclip-personal', 'deploy name')
assert(deploy.dig('spec', 'replicas') == 1, 'deploy replicas 1')
assert(deploy.dig('spec', 'strategy', 'type') == 'Recreate', 'deploy Recreate')
podspec = deploy.dig('spec', 'template', 'spec')
assert(podspec['automountServiceAccountToken'] == false, 'automountServiceAccountToken false')
assert(podspec.dig('securityContext', 'seccompProfile', 'type') == 'RuntimeDefault', 'seccomp RuntimeDefault')
app = podspec['containers'].first
assert(app['image'] == 'ghcr.io/paperclipai/paperclip:sha-d554c47@sha256:f203471a38603026aa861efef343c56a2785202f5f83da440a48f85e5a949a49', 'app image pinned')
assert(app.dig('securityContext', 'allowPrivilegeEscalation') == false, 'app allowPrivilegeEscalation false')
assert(!app.dig('securityContext').key?('capabilities'), 'app must not drop capabilities')
assert(app.dig('ports', 0, 'containerPort') == 3100, 'app containerPort 3100')

env = app['env'].each_with_object({}) { |e, h| h[e['name']] = e }
{
  'PORT' => '3100',
  'SERVE_UI' => 'true',
  'PAPERCLIP_HOME' => '/paperclip',
  'PAPERCLIP_DEPLOYMENT_MODE' => 'authenticated',
  'PAPERCLIP_DEPLOYMENT_EXPOSURE' => 'private',
  'PAPERCLIP_MIGRATION_AUTO_APPLY' => 'true',
  'PAPERCLIP_PUBLIC_URL' => 'https://paperclip.theramirez.casa',
  'PAPERCLIP_AUTH_DISABLE_SIGN_UP' => 'true',
  'PAPERCLIP_AUTH_RATE_LIMIT_ENABLED' => 'true',
  'PAPERCLIP_AGENT_JWT_DISABLE_LEGACY_FALLBACK' => 'true'
}.each do |k, v|
  assert(env[k].dig('value') == v, "env #{k}=#{v}")
end
%w[DATABASE_URL BETTER_AUTH_SECRET PAPERCLIP_TOOL_ACTION_SIGNING_SECRET PAPERCLIP_AGENT_JWT_SECRET].each do |k|
  assert(env[k].dig('valueFrom', 'secretKeyRef', 'name') == 'paperclip-personal', "#{k} secretKeyRef name")
  assert(env[k].dig('valueFrom', 'secretKeyRef', 'key') == k, "#{k} secretKeyRef key")
end
%w[startupProbe readinessProbe livenessProbe].each do |probe|
  assert(app.dig(probe, 'httpGet', 'path') == '/api/health', "#{probe} path /api/health")
  assert(app.dig(probe, 'httpGet', 'port') == 'http', "#{probe} port http")
  assert(app.dig(probe, 'httpGet', 'httpHeaders', 0, 'name') == 'Host', "#{probe} has Host header")
  assert(app.dig(probe, 'httpGet', 'httpHeaders', 0, 'value') == 'paperclip.theramirez.casa', "#{probe} Host header = hostname")
end
assert(app.dig('startupProbe', 'failureThreshold') == 60, 'startupProbe failureThreshold 60')
assert(app.dig('startupProbe', 'periodSeconds') == 5, 'startupProbe periodSeconds 5')
assert(app.dig('volumeMounts', 0, 'mountPath') == '/paperclip', 'mountPath /paperclip')

route = by_kind['HTTPRoute'].first
assert(route.dig('metadata', 'name') == 'paperclip-personal', 'route name')
pref = route.dig('spec', 'parentRefs', 0)
assert(pref['name'] == 'home', 'route gateway home')
assert(pref['namespace'] == 'kgateway-system', 'route gateway namespace kgateway-system')
assert(pref['sectionName'] == 'https', 'route section https')
assert(route.dig('spec', 'hostnames') == ['paperclip.theramirez.casa'], 'route hostname')
assert(route.dig('spec', 'rules', 0, 'backendRefs', 0, 'name') == 'paperclip-personal', 'route backend name')
assert(route.dig('spec', 'rules', 0, 'backendRefs', 0, 'port') == 3100, 'route backend port 3100')

%w[Ingress Gateway].each { |k| assert(!by_kind.key?(k), "forbidden #{k} present") }
raw = File.read(file)
%w[hostPath privileged LoadBalancer docker.sock cloudflared tunnel].each do |bad|
  assert(!raw.include?(bad), "forbidden token #{bad} present")
end

puts 'ok   - claimed render assertions passed (route, https, signup closed, JWT ref, PSA)'
RUBY

# --- 9. secret-file validator (task precondition semantics) ------------------
cat > "$TMP/secret-good.yaml" <<'SECRETS'
secrets:
  postgres:
    password: real-password-value-000000000000000000000000000000000000
  betterAuthSecret: real-better-auth-secret-111111111111111111111111111111
  toolActionSigningSecret: real-tool-signing-secret-22222222222222222222222222
  agentJwtSecret: real-agent-jwt-secret-333333333333333333333333333333333
SECRETS
chmod 600 "$TMP/secret-good.yaml"
if ! tests/validate-secret-file.sh "$TMP/secret-good.yaml"; then
  fail "valid 0600 secret file without testMode should pass validation"
fi
pass "validate-secret-file: 0600 + no testMode passes"

for label in 'testMode: true' 'testMode: false' 'testMode: "false"' 'testMode: null'; do
  cat > "$TMP/secret-tm.yaml" <<EOF
$label
secrets:
  postgres:
    password: real-password-value-000000000000000000000000000000000000
  betterAuthSecret: real-better-auth-secret-111111111111111111111111111111
  toolActionSigningSecret: real-tool-signing-secret-22222222222222222222222222
  agentJwtSecret: real-agent-jwt-secret-333333333333333333333333333333333
EOF
  chmod 600 "$TMP/secret-tm.yaml"
  if tests/validate-secret-file.sh "$TMP/secret-tm.yaml"; then
    fail "secret file with $label should be rejected"
  fi
done
pass "validate-secret-file: testMode key rejected (true/false/\"false\"/null)"

cat > "$TMP/secret-badmode.yaml" <<'SECRETS'
secrets:
  postgres:
    password: real-password-value-000000000000000000000000000000000000
  betterAuthSecret: real-better-auth-secret-111111111111111111111111111111
  toolActionSigningSecret: real-tool-signing-secret-22222222222222222222222222
  agentJwtSecret: real-agent-jwt-secret-333333333333333333333333333333333
SECRETS
chmod 644 "$TMP/secret-badmode.yaml"
if tests/validate-secret-file.sh "$TMP/secret-badmode.yaml"; then
  fail "secret file with mode 0644 should be rejected"
fi
pass "validate-secret-file: non-0600 mode rejected"

# --- 10. Taskfile bootstrap-invite + rollback-on-failure semantics -----------
TF="../../Taskfile.yaml"
if [ ! -f "$TF" ]; then
  fail "Taskfile.yaml not found relative to the chart dir"
fi

# bootstrap-invite must use the ephemeral /tmp config + trap + invite URL
# extraction, must NOT pass --db-url (DATABASE_URL is inherited from the pod
# env), and must never write /paperclip/instances/default/config.json.
grep -qF -- '--config' "$TF" || fail "bootstrap-invite missing --config"
if grep -qF -- '--db-url' "$TF"; then
  fail "bootstrap-invite must not pass --db-url (DATABASE_URL is inherited from the pod env)"
fi
grep -qF -- '/tmp/pcp-bootstrap-config' "$TF" || fail "bootstrap-invite missing /tmp config"
grep -qF -- '"version":1' "$TF" || fail "bootstrap-invite config version must be integer 1"
grep -qF -- '"source":"configure"' "$TF" || fail "bootstrap-invite config source must be configure"
grep -qF -- '"database":{"mode":"postgres"}' "$TF" || fail "bootstrap-invite config database.mode must be postgres"
grep -qF -- '"logging":{"mode":"file"}' "$TF" || fail "bootstrap-invite config logging.mode must be file"
grep -qF -- 'trap' "$TF" || fail "bootstrap-invite missing trap cleanup"
grep -qF -- 'http://localhost:3100/invite/pcp_bootstrap_' "$TF" || fail "bootstrap-invite missing invite URL extraction"
if grep -qF -- '/paperclip/instances/default/config.json' "$TF"; then
  fail "bootstrap-invite must not write /paperclip/instances/default/config.json"
fi
pass "bootstrap-invite uses ephemeral /tmp config (configure/file/postgres) + trap + invite URL extraction, no --db-url"

# Paperclip mutating upgrades use --rollback-on-failure (Helm 4), not --atomic.
pc_upgrades="$(grep -cF 'helm upgrade --install {{.PAPERCLIP_RELEASE}}' "$TF")"
[ "$pc_upgrades" -ge 2 ] || fail "expected >=2 paperclip helm upgrade commands (got $pc_upgrades)"
pc_atomic="$(grep -F 'helm upgrade --install {{.PAPERCLIP_RELEASE}}' "$TF" | grep -cF -- '--atomic' || true)"
pc_rollback="$(grep -F 'helm upgrade --install {{.PAPERCLIP_RELEASE}}' "$TF" | grep -cF -- '--rollback-on-failure' || true)"
[ "$pc_atomic" -eq 0 ] || fail "paperclip upgrade tasks still use --atomic"
[ "$pc_rollback" -eq 2 ] || fail "expected 2 paperclip upgrade tasks to use --rollback-on-failure (got $pc_rollback)"
pass "paperclip upgrade tasks use --rollback-on-failure (not --atomic)"

# paperclip:verify must use a deterministic DNS check + --resolve health curl so
# a stale macOS negative resolver cache can't hide the route/app.
grep -qF -- 'dig +short paperclip.theramirez.casa A' "$TF" || fail "verify missing dig DNS check"
grep -qF -- '--resolve paperclip.theramirez.casa:443:192.168.1.83' "$TF" || fail "verify missing --resolve health curl"
pass "paperclip:verify uses dig DNS check + --resolve health curl"

echo
echo "ALL PAPERCLIP CONTRACT TESTS PASSED"
