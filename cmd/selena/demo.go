package main

import (
	"fmt"
	"os"
	"strings"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
)

// runDemo lowers the high-level DirectionalDiffuse material through the full
// pipeline (HIR -> lower -> LIR + binding layout), emits WGSL + GLSL ES 3.00,
// and bakes them — together with the generated binding descriptor — into a
// self-contained WebGPU + WebGL2 render harness. The harness packs its uniform
// buffer **from the descriptor's byte offsets**, not hand-written indices: M1's
// proof that the std140 layout pain is gone on the host side too.
func runDemo(out string) error {
	mod, layout, err := lower.Lower(hir.DirectionalDiffuse())
	if err != nil {
		return fmt.Errorf("lower: %w", err)
	}
	wgslSrc, err := wgsl.Emit(mod)
	if err != nil {
		return fmt.Errorf("emit wgsl: %w", err)
	}
	glesVert, glesFrag, err := gles.Emit(mod)
	if err != nil {
		return fmt.Errorf("emit gles: %w", err)
	}
	desc, err := layout.JSON()
	if err != nil {
		return fmt.Errorf("descriptor: %w", err)
	}

	page := demoHTML
	page = strings.Replace(page, "__WGSL__", strings.TrimSpace(wgslSrc), 1)
	page = strings.Replace(page, "__GLES_VERT__", strings.TrimSpace(glesVert), 1)
	page = strings.Replace(page, "__GLES_FRAG__", strings.TrimSpace(glesFrag), 1)
	page = strings.Replace(page, "__DESCRIPTOR__", strings.TrimSpace(desc), 1)

	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("wrote %s — open it in Chrome (WebGPU needs Chrome 113+)\n", out)
	return nil
}

// demoHTML is the harness template. Shaders + descriptor are injected into
// <script> blocks; the JS uses no backticks so the template fits a Go raw string.
const demoHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>selena · DirectionalDiffuse — one IR, two GPUs, generated bindings</title>
<style>
  :root { color-scheme: dark; }
  body { margin: 0; background: #0b0b12; color: #e7e7f0;
         font: 14px/1.5 ui-sans-serif, system-ui, sans-serif; }
  header { padding: 20px 24px 8px; }
  h1 { margin: 0 0 4px; font-size: 18px; font-weight: 650; }
  .sub { color: #9aa; margin: 0; max-width: 760px; }
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
  <h1>🌒 selena · DirectionalDiffuse</h1>
  <p class="sub">High-level material &rarr; lowered IR &rarr; emitted shaders <b>and</b> a generated
  binding descriptor. The uniform buffer below is packed straight from the descriptor's std140
  byte offsets — no hand-written layout on either side of the GPU.</p>
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

<details open><summary>generated binding descriptor (drives the host packing)</summary><pre id="show-desc"></pre></details>
<details><summary>emitted WGSL</summary><pre id="show-wgsl"></pre></details>
<details><summary>emitted GLSL ES 3.00 (vertex)</summary><pre id="show-glesv"></pre></details>
<details><summary>emitted GLSL ES 3.00 (fragment)</summary><pre id="show-glesf"></pre></details>

<script type="x-shader/wgsl" id="src-wgsl">__WGSL__</script>
<script type="x-shader/glsl" id="src-glesv">__GLES_VERT__</script>
<script type="x-shader/glsl" id="src-glesf">__GLES_FRAG__</script>
<script type="application/json" id="src-desc">__DESCRIPTOR__</script>

<script>
"use strict";
function $(id){ return document.getElementById(id); }
function setStatus(id, msg, cls){ var el=$(id); el.textContent=msg; el.className="status "+(cls||""); }

var WGSL = $("src-wgsl").textContent.trim();
var GLES_VERT = $("src-glesv").textContent.trim();
var GLES_FRAG = $("src-glesf").textContent.trim();
var DESC = JSON.parse($("src-desc").textContent);
$("show-desc").textContent = JSON.stringify(DESC, null, 2);
$("show-wgsl").textContent = WGSL;
$("show-glesv").textContent = GLES_VERT;
$("show-glesf").textContent = GLES_FRAG;

// --- material param values the host supplies (selena's signature glow) ---
var MATERIAL = { baseColor: [0.78, 0.42, 0.98], light_dir: [0.4, 0.85, 0.6], light_ambient: 0.16 };
function uniformValues(sc){
  // mvp + normalMatrix come from the runtime transform; the rest are material params.
  var v = { mvp: sc.mvp, normalMatrix: sc.nrm };
  for (var k in MATERIAL) v[k] = MATERIAL[k];
  return v;
}

// --- type metadata derived from the descriptor's type strings ---
function floatCount(t){ return {float:1, vec2:2, vec3:3, vec4:4, mat3:9, mat4:16}[t]; }
function byteSize(t){ return {float:4, vec2:8, vec3:12, vec4:16, mat3:48, mat4:64}[t]; } // std140
function attrFloats(t){ return {vec2:2, vec3:3, vec4:4}[t]; }

// --- the std140 packer: descriptor offsets -> a uniform buffer. General; no
//     per-material hand-coding. mat3 is the painful case (3 cols, 16B stride). ---
function packUniforms(desc, values){
  var f32 = new Float32Array(desc.uniformBlock.size / 4);
  desc.uniformBlock.fields.forEach(function(f){
    var v = values[f.name], base = f.offset / 4;
    if (f.type === "float") { f32[base] = v; }
    else if (f.type === "mat3") {
      for (var c = 0; c < 3; c++){ f32[base+c*4]=v[c*3]; f32[base+c*4+1]=v[c*3+1]; f32[base+c*4+2]=v[c*3+2]; }
    } else { f32.set(v.slice(0, floatCount(f.type)), base); }
  });
  return f32;
}

// --- column-major mat4 / mat3 helpers ---
function mat4Mul(a, b){
  var o = new Array(16);
  for (var c=0;c<4;c++) for (var r=0;r<4;r++)
    o[c*4+r] = a[0*4+r]*b[c*4+0] + a[1*4+r]*b[c*4+1] + a[2*4+r]*b[c*4+2] + a[3*4+r]*b[c*4+3];
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
  var proj = mat4Perspective(Math.PI/4, 1, 0.1, 100), view = mat4Translate(0,0,-4);
  var model = mat4Mul(mat4RotateY(t*0.8), mat4RotateX(t*0.55));
  return { mvp: mat4Mul(proj, mat4Mul(view, model)), nrm: mat3FromMat4(model) };
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
  for (var f=0; f<faces.length; f++) for (var i=0; i<tri.length; i++){
    var p = faces[f].v[tri[i]], n = faces[f].n;
    data.push(p[0],p[1],p[2], n[0],n[1],n[2]);
  }
  return new Float32Array(data);
}
var CUBE = buildCube();
var VCOUNT = CUBE.length / 6;
var VSTRIDE = 0; DESC.attributes.forEach(function(a){ VSTRIDE += attrFloats(a.type)*4; });

// ============================ WebGPU (WGSL) ============================
async function initWebGPU(){
  if (!navigator.gpu){ setStatus("gpustatus","WebGPU unavailable (need Chrome 113+)","err"); return; }
  var adapter = await navigator.gpu.requestAdapter();
  if (!adapter){ setStatus("gpustatus","no WebGPU adapter","err"); return; }
  var device = await adapter.requestDevice();
  if (device.addEventListener) device.addEventListener("uncapturederror", function(e){
    setStatus("gpustatus","runtime error: "+e.error.message,"err"); });

  var canvas = $("gpu"), ctx = canvas.getContext("webgpu");
  var format = navigator.gpu.getPreferredCanvasFormat();
  ctx.configure({ device: device, format: format, alphaMode: "opaque" });

  var vbuf = device.createBuffer({ size: CUBE.byteLength, usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST });
  device.queue.writeBuffer(vbuf, 0, CUBE);
  var ubuf = device.createBuffer({ size: DESC.uniformBlock.size, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });

  // vertex layout derived from the descriptor's attributes
  var off = 0, wgslAttrs = [];
  DESC.attributes.forEach(function(a){
    wgslAttrs.push({ shaderLocation: a.location, offset: off, format: "float32x"+attrFloats(a.type) });
    off += attrFloats(a.type) * 4;
  });

  var module = device.createShaderModule({ code: WGSL });
  var info = await module.getCompilationInfo();
  for (var i=0;i<info.messages.length;i++) if (info.messages[i].type==="error"){
    setStatus("gpustatus","WGSL compile error: "+info.messages[i].message,"err"); return; }

  var pipeline = device.createRenderPipeline({
    layout: "auto",
    vertex: { module: module, entryPoint: "vertexMain", buffers: [{ arrayStride: VSTRIDE, attributes: wgslAttrs }] },
    fragment: { module: module, entryPoint: "fragmentMain", targets: [{ format: format }] },
    primitive: { topology: "triangle-list", cullMode: "back", frontFace: "ccw" }
  });
  var bind = device.createBindGroup({ layout: pipeline.getBindGroupLayout(DESC.wgsl.group),
    entries: [{ binding: DESC.wgsl.binding, resource: { buffer: ubuf } }] });

  setStatus("gpustatus","WebGPU OK · packed "+DESC.uniformBlock.size+"B from descriptor","ok");

  function frame(now){
    device.queue.writeBuffer(ubuf, 0, packUniforms(DESC, uniformValues(scene(now*0.001))));
    var enc = device.createCommandEncoder();
    var pass = enc.beginRenderPass({ colorAttachments: [{
      view: ctx.getCurrentTexture().createView(),
      clearValue: { r:0.03, g:0.03, b:0.05, a:1 }, loadOp: "clear", storeOp: "store" }] });
    pass.setPipeline(pipeline); pass.setBindGroup(DESC.wgsl.group, bind);
    pass.setVertexBuffer(0, vbuf); pass.draw(VCOUNT); pass.end();
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
    var sh = gl.createShader(type); gl.shaderSource(sh, src); gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)){ setStatus("glstatus", label+" error: "+gl.getShaderInfoLog(sh), "err"); return null; }
    return sh;
  }
  var vs = compile(gl.VERTEX_SHADER, GLES_VERT, "vertex"), fs = compile(gl.FRAGMENT_SHADER, GLES_FRAG, "fragment");
  if (!vs || !fs) return;
  var prog = gl.createProgram(); gl.attachShader(prog, vs); gl.attachShader(prog, fs); gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)){ setStatus("glstatus","link error: "+gl.getProgramInfoLog(prog),"err"); return; }

  var vbo = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, vbo); gl.bufferData(gl.ARRAY_BUFFER, CUBE, gl.STATIC_DRAW);
  // attribute pointers derived from the descriptor
  var off = 0, ptrs = [];
  DESC.attributes.forEach(function(a){
    ptrs.push({ loc: gl.getAttribLocation(prog, a.name), n: attrFloats(a.type), offset: off }); off += attrFloats(a.type)*4;
  });
  // uniform setters keyed by the descriptor
  var setters = { float: "uniform1f", vec2: "uniform2fv", vec3: "uniform3fv", vec4: "uniform4fv" };
  gl.enable(gl.CULL_FACE); gl.cullFace(gl.BACK); gl.frontFace(gl.CCW);
  gl.clearColor(0.03, 0.03, 0.05, 1);
  setStatus("glstatus","WebGL2 OK · "+gl.getParameter(gl.VERSION),"ok");

  function frame(now){
    var vals = uniformValues(scene(now*0.001));
    gl.viewport(0,0,canvas.width,canvas.height); gl.clear(gl.COLOR_BUFFER_BIT); gl.useProgram(prog);
    DESC.uniformBlock.fields.forEach(function(f){
      var loc = gl.getUniformLocation(prog, f.name), v = vals[f.name];
      if (f.type === "mat3") gl.uniformMatrix3fv(loc, false, v);
      else if (f.type === "mat4") gl.uniformMatrix4fv(loc, false, v);
      else gl[setters[f.type]](loc, v);
    });
    gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
    ptrs.forEach(function(p){ gl.enableVertexAttribArray(p.loc); gl.vertexAttribPointer(p.loc, p.n, gl.FLOAT, false, VSTRIDE, p.offset); });
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
