package main

import (
	"fmt"
	"os"
	"strings"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/ir"
)

// runDemo writes a self-contained HTML proof harness that renders the built-in
// DirectionalDiffuse material two ways — WebGPU (the emitted WGSL) and WebGL2
// (the emitted GLSL ES 3.00) — side by side on a spinning lit cube. The shaders
// baked into the page are the real emitter output, so opening it in a browser
// proves the IR -> emitter -> GPU loop end to end.
func runDemo(out string) error {
	m := ir.DirectionalDiffuse()
	wgslSrc, err := wgsl.Emit(m)
	if err != nil {
		return fmt.Errorf("emit wgsl: %w", err)
	}
	glesVert, glesFrag, err := gles.Emit(m)
	if err != nil {
		return fmt.Errorf("emit gles: %w", err)
	}

	page := demoHTML
	page = strings.Replace(page, "__WGSL__", strings.TrimSpace(wgslSrc), 1)
	page = strings.Replace(page, "__GLES_VERT__", strings.TrimSpace(glesVert), 1)
	page = strings.Replace(page, "__GLES_FRAG__", strings.TrimSpace(glesFrag), 1)

	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("wrote %s — open it in Chrome (WebGPU needs Chrome 113+)\n", out)
	return nil
}

// demoHTML is the harness template. The shaders are injected into x-shader
// <script> blocks (raw text, no JS string-literal escaping needed); the JS uses
// no backticks so the whole template can live in a Go raw string.
const demoHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>selena · DirectionalDiffuse — one IR, two GPUs</title>
<style>
  :root { color-scheme: dark; }
  body { margin: 0; background: #0b0b12; color: #e7e7f0;
         font: 14px/1.5 ui-sans-serif, system-ui, sans-serif; }
  header { padding: 20px 24px 8px; }
  h1 { margin: 0 0 4px; font-size: 18px; font-weight: 650; }
  h1 .moon { opacity: .8; }
  .sub { color: #9aa; margin: 0; }
  .row { display: flex; flex-wrap: wrap; gap: 24px; padding: 16px 24px; }
  figure { margin: 0; }
  figcaption { font-size: 12px; color: #b9b9cc; margin-bottom: 6px; }
  canvas { width: 360px; height: 360px; border-radius: 10px;
           background: #07070d; box-shadow: 0 0 0 1px #1c1c2a; display: block; }
  .status { font-size: 12px; margin-top: 6px; min-height: 16px; }
  .ok { color: #79e08a; }
  .err { color: #ff7a7a; white-space: pre-wrap; }
  details { padding: 8px 24px 28px; max-width: 760px; }
  summary { cursor: pointer; color: #b9b9cc; }
  pre { background: #07070d; border: 1px solid #1c1c2a; border-radius: 8px;
        padding: 12px; overflow: auto; font-size: 12px; color: #cdd; }
</style>
</head>
<body>
<header>
  <h1><span class="moon">🌒</span> selena · DirectionalDiffuse</h1>
  <p class="sub">One IR &rarr; one emitter per backend &rarr; real GPU pipelines. The shaders below are verbatim emitter output.</p>
</header>

<div class="row">
  <figure>
    <figcaption>WebGPU · WGSL &nbsp;(browser + GoSX desktop)</figcaption>
    <canvas id="gpu" width="360" height="360"></canvas>
    <div id="gpustatus" class="status">starting…</div>
  </figure>
  <figure>
    <figcaption>WebGL2 · GLSL ES 3.00 &nbsp;(browser + Android GLES3)</figcaption>
    <canvas id="gl" width="360" height="360"></canvas>
    <div id="glstatus" class="status">starting…</div>
  </figure>
</div>

<details><summary>emitted WGSL</summary><pre id="show-wgsl"></pre></details>
<details><summary>emitted GLSL ES 3.00 (vertex)</summary><pre id="show-glesv"></pre></details>
<details><summary>emitted GLSL ES 3.00 (fragment)</summary><pre id="show-glesf"></pre></details>

<script type="x-shader/wgsl" id="src-wgsl">__WGSL__</script>
<script type="x-shader/glsl" id="src-glesv">__GLES_VERT__</script>
<script type="x-shader/glsl" id="src-glesf">__GLES_FRAG__</script>

<script>
"use strict";
function $(id){ return document.getElementById(id); }
function setStatus(id, msg, cls){ var el=$(id); el.textContent=msg; el.className="status "+(cls||""); }

var WGSL = $("src-wgsl").textContent.trim();
var GLES_VERT = $("src-glesv").textContent.trim();
var GLES_FRAG = $("src-glesf").textContent.trim();
$("show-wgsl").textContent = WGSL;
$("show-glesv").textContent = GLES_VERT;
$("show-glesf").textContent = GLES_FRAG;

// --- material constants (selena's signature glow) ---
var baseColor = [0.78, 0.42, 0.98];
var lightDir  = [0.4, 0.85, 0.6];
var ambient   = 0.16;

// --- column-major mat4 / mat3 helpers ---
function mat4Mul(a, b){
  var o = new Array(16);
  for (var c=0;c<4;c++) for (var r=0;r<4;r++){
    o[c*4+r] = a[0*4+r]*b[c*4+0] + a[1*4+r]*b[c*4+1] + a[2*4+r]*b[c*4+2] + a[3*4+r]*b[c*4+3];
  }
  return o;
}
function mat4Perspective(fovy, aspect, near, far){
  var f = 1/Math.tan(fovy/2), nf = 1/(near-far);
  return [f/aspect,0,0,0, 0,f,0,0, 0,0,(far+near)*nf,-1, 0,0,2*far*near*nf,0];
}
function mat4Translate(x,y,z){ return [1,0,0,0, 0,1,0,0, 0,0,1,0, x,y,z,1]; }
function mat4RotateY(a){ var c=Math.cos(a), s=Math.sin(a); return [c,0,-s,0, 0,1,0,0, s,0,c,0, 0,0,0,1]; }
function mat4RotateX(a){ var c=Math.cos(a), s=Math.sin(a); return [1,0,0,0, 0,c,s,0, 0,-s,c,0, 0,0,0,1]; }
function mat3FromMat4(m){ return [m[0],m[1],m[2], m[4],m[5],m[6], m[8],m[9],m[10]]; }

function scene(t){
  var proj  = mat4Perspective(Math.PI/4, 1, 0.1, 100);
  var view  = mat4Translate(0,0,-4);
  var model = mat4Mul(mat4RotateY(t*0.8), mat4RotateX(t*0.55));
  var mvp   = mat4Mul(proj, mat4Mul(view, model));
  return { mvp: mvp, nrm: mat3FromMat4(model) };
}

// --- cube: 36 verts, interleaved position(3)+normal(3), CCW-outward ---
function buildCube(){
  var s = 0.85;
  var faces = [
    {n:[0,0,1],  v:[[-s,-s,s],[s,-s,s],[s,s,s],[-s,s,s]]},
    {n:[0,0,-1], v:[[s,-s,-s],[-s,-s,-s],[-s,s,-s],[s,s,-s]]},
    {n:[1,0,0],  v:[[s,-s,s],[s,-s,-s],[s,s,-s],[s,s,s]]},
    {n:[-1,0,0], v:[[-s,-s,-s],[-s,-s,s],[-s,s,s],[-s,s,-s]]},
    {n:[0,1,0],  v:[[-s,s,s],[s,s,s],[s,s,-s],[-s,s,-s]]},
    {n:[0,-1,0], v:[[-s,-s,-s],[s,-s,-s],[s,-s,s],[-s,-s,s]]}
  ];
  var data = [], tri = [0,1,2, 0,2,3];
  for (var f=0; f<faces.length; f++){
    for (var i=0; i<tri.length; i++){
      var p = faces[f].v[tri[i]], n = faces[f].n;
      data.push(p[0],p[1],p[2], n[0],n[1],n[2]);
    }
  }
  return new Float32Array(data);
}
var CUBE = buildCube();
var VCOUNT = CUBE.length / 6;

// ============================ WebGPU (WGSL) ============================
async function initWebGPU(){
  if (!navigator.gpu){ setStatus("gpustatus","WebGPU unavailable (need Chrome 113+ / WebGPU enabled)","err"); return; }
  var adapter = await navigator.gpu.requestAdapter();
  if (!adapter){ setStatus("gpustatus","no WebGPU adapter","err"); return; }
  var device = await adapter.requestDevice();
  device.addEventListener && device.addEventListener("uncapturederror", function(e){
    setStatus("gpustatus","WGSL/runtime error: "+e.error.message,"err");
  });

  var canvas = $("gpu"), ctx = canvas.getContext("webgpu");
  var format = navigator.gpu.getPreferredCanvasFormat();
  ctx.configure({ device: device, format: format, alphaMode: "opaque" });

  var vbuf = device.createBuffer({ size: CUBE.byteLength, usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST });
  device.queue.writeBuffer(vbuf, 0, CUBE);

  // Uniforms layout (WGSL uniform address space, bytes):
  // mvp mat4x4 @0 (64) | normalMatrix mat3x3 @64 (48, cols padded to 16) |
  // baseColor vec3 @112 | lightDir vec3 @128 | ambient f32 @140 | size 144.
  var ubuf = device.createBuffer({ size: 144, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });

  var module = device.createShaderModule({ code: WGSL });
  var info = await module.getCompilationInfo();
  for (var i=0;i<info.messages.length;i++){ if (info.messages[i].type==="error"){
    setStatus("gpustatus","WGSL compile error: "+info.messages[i].message,"err"); return; } }

  var pipeline = device.createRenderPipeline({
    layout: "auto",
    vertex: { module: module, entryPoint: "vertexMain", buffers: [{
      arrayStride: 24,
      attributes: [ { shaderLocation:0, offset:0, format:"float32x3" },
                    { shaderLocation:1, offset:12, format:"float32x3" } ]
    }]},
    fragment: { module: module, entryPoint: "fragmentMain", targets: [{ format: format }] },
    primitive: { topology: "triangle-list", cullMode: "back", frontFace: "ccw" }
  });
  var bind = device.createBindGroup({ layout: pipeline.getBindGroupLayout(0),
    entries: [{ binding: 0, resource: { buffer: ubuf } }] });

  setStatus("gpustatus","WebGPU OK · "+(adapter.info && adapter.info.vendor ? adapter.info.vendor : "rendering"),"ok");

  function frame(now){
    var sc = scene(now*0.001);
    var u = new Float32Array(36);
    u.set(sc.mvp, 0);
    u[16]=sc.nrm[0]; u[17]=sc.nrm[1]; u[18]=sc.nrm[2];   // col0 @64
    u[20]=sc.nrm[3]; u[21]=sc.nrm[4]; u[22]=sc.nrm[5];   // col1 @80
    u[24]=sc.nrm[6]; u[25]=sc.nrm[7]; u[26]=sc.nrm[8];   // col2 @96
    u[28]=baseColor[0]; u[29]=baseColor[1]; u[30]=baseColor[2]; // @112
    u[32]=lightDir[0];  u[33]=lightDir[1];  u[34]=lightDir[2];  // @128
    u[35]=ambient;                                             // @140
    device.queue.writeBuffer(ubuf, 0, u);

    var enc = device.createCommandEncoder();
    var pass = enc.beginRenderPass({ colorAttachments: [{
      view: ctx.getCurrentTexture().createView(),
      clearValue: { r:0.03, g:0.03, b:0.05, a:1 }, loadOp: "clear", storeOp: "store" }] });
    pass.setPipeline(pipeline);
    pass.setBindGroup(0, bind);
    pass.setVertexBuffer(0, vbuf);
    pass.draw(VCOUNT);
    pass.end();
    device.queue.submit([enc.finish()]);
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

// ============================ WebGL2 (GLSL ES 3.00) ============================
function initWebGL(){
  var canvas = $("gl"), gl = canvas.getContext("webgl2");
  if (!gl){ setStatus("glstatus","WebGL2 unavailable","err"); return; }

  function compile(type, src, label){
    var sh = gl.createShader(type);
    gl.shaderSource(sh, src); gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)){
      setStatus("glstatus", label+" error: "+gl.getShaderInfoLog(sh), "err");
      return null;
    }
    return sh;
  }
  var vs = compile(gl.VERTEX_SHADER, GLES_VERT, "vertex");
  var fs = compile(gl.FRAGMENT_SHADER, GLES_FRAG, "fragment");
  if (!vs || !fs) return;
  var prog = gl.createProgram();
  gl.attachShader(prog, vs); gl.attachShader(prog, fs); gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)){
    setStatus("glstatus","link error: "+gl.getProgramInfoLog(prog),"err"); return;
  }

  var vbo = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
  gl.bufferData(gl.ARRAY_BUFFER, CUBE, gl.STATIC_DRAW);
  var posLoc = gl.getAttribLocation(prog, "position");
  var nrmLoc = gl.getAttribLocation(prog, "normal");
  var uMVP = gl.getUniformLocation(prog, "mvp");
  var uNrm = gl.getUniformLocation(prog, "normalMatrix");
  var uCol = gl.getUniformLocation(prog, "baseColor");
  var uLit = gl.getUniformLocation(prog, "lightDir");
  var uAmb = gl.getUniformLocation(prog, "ambient");

  gl.enable(gl.CULL_FACE); gl.cullFace(gl.BACK); gl.frontFace(gl.CCW);
  gl.clearColor(0.03, 0.03, 0.05, 1);
  setStatus("glstatus","WebGL2 OK · "+gl.getParameter(gl.VERSION),"ok");

  function frame(now){
    var sc = scene(now*0.001);
    gl.viewport(0,0,canvas.width,canvas.height);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(prog);
    gl.uniformMatrix4fv(uMVP, false, sc.mvp);
    gl.uniformMatrix3fv(uNrm, false, sc.nrm);
    gl.uniform3fv(uCol, baseColor);
    gl.uniform3fv(uLit, lightDir);
    gl.uniform1f(uAmb, ambient);
    gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
    gl.enableVertexAttribArray(posLoc);
    gl.vertexAttribPointer(posLoc, 3, gl.FLOAT, false, 24, 0);
    gl.enableVertexAttribArray(nrmLoc);
    gl.vertexAttribPointer(nrmLoc, 3, gl.FLOAT, false, 24, 12);
    gl.drawArrays(gl.TRIANGLES, 0, VCOUNT);
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

initWebGPU().catch(function(e){ setStatus("gpustatus","WebGPU init failed: "+e.message,"err"); });
initWebGL();
</script>
</body>
</html>
`
