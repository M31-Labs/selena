package validate

import (
	"context"
	_ "embed"
	"encoding/json"
	"m31labs.dev/selena"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

//go:embed specular_webgl.html
var specularWebGLHTML string

// Chromium validates the actual WEBGL_draw_buffers extension that glslang does
// not implement. The fixture also reads both attachments: surface discard must
// leave both clear, early returns must still write the companion, alpha must
// match, and a bright material without specular must contribute exactly zero.
func TestSpecularMRTWebGL1(t *testing.T) {
	chrome := os.Getenv("SELENA_CHROME")
	if chrome == "" {
		t.Skip("set SELENA_CHROME to run real WebGL1 MRT compile/link/pixel coverage")
	}
	fixtures := []string{
		`material M {
 surface(g)->vec4 {
  if (g.uv.x<0.25) {discard}
  if (g.uv.x<0.5) {return vec4f(0.2,0.3,0.4,0.25)}
  return vec4f(0.2,0.3,0.4,0.5)
 }
 specular(h)->color {if(h.uv.x<0.5){return rgb(0.8,0.1,0.2)} return rgb(0.1,0.8,0.2)}
 }`,
		`material Zero {surface(g)->vec4{return vec4f(1.0,1.0,1.0,0.5)}}`,
		`material Texture {
 param image: texture2d
 param env: textureCube
 surface(g)->color{return rgb(0.4,0.3,0.2)}
 specular(g)->color{return sample(image,g.uv).rgb*fwidth(g.uv.x)+sampleCubeLevel(env,g.worldNormal,0.0).rgb}
 }`,
	}
	fixtures = append(fixtures, `material Fused {
 surface(g)->vec4 {let a=rgb(0.2,0.3,0.4) let value=a*1.0 return vec4f(value,0.5)}
 specular(g)->color {let a=rgb(0.2,0.3,0.4) let value=a*2.0 return value}
 }`)
	var shaders []selena.Artifact
	for _, src := range fixtures {
		r, err := selena.Compile([]byte(src), selena.CompileOptions{SpecularMRT: true, Targets: []selena.Target{selena.TargetGLSL}})
		if err != nil {
			t.Fatal(err)
		}
		shaders = append(shaders, r.Artifacts[0])
	}
	data, err := json.Marshal(shaders)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "specular.html")
	if err := os.WriteFile(path, []byte(strings.Replace(specularWebGLHTML, "SHADERS_JSON", string(data), 1)), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, chrome, "--headless", "--no-sandbox", "--enable-unsafe-swiftshader", "--use-gl=angle", "--use-angle=swiftshader", "--disable-dev-shm-usage", "--user-data-dir="+filepath.Join(dir, "profile"), "--dump-dom", "file://"+path).CombinedOutput()
	if err != nil || !strings.Contains(string(out), `<pre id="result">PASS: 4 programs, surface coverage, early return, specular RGB, zero fallback</pre>`) {
		t.Fatalf("WebGL1 MRT proof: %v\n%s", err, out)
	}
}
