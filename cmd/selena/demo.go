package main

import (
	"fmt"
	"os"
	"strings"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
)

// runDemo lowers a material (directional-diffuse | textured) through the full
// pipeline and bakes the emitted WGSL + GLSL ES 3.00 + GLSL ES 1.00 and the
// generated binding descriptor into a self-contained WebGPU + WebGL render
// harness. The harness packs uniforms and binds textures straight from the
// descriptor, and degrades gracefully: it tries a hardware WebGPU adapter then a
// software one, and WebGL2 then WebGL1 — so it renders even where GPU access is
// limited.
func runDemo(out, material string) error {
	var m hir.Material
	switch material {
	case "", "directional-diffuse":
		m = hir.DirectionalDiffuse()
	case "textured":
		m = hir.Textured()
	default:
		return fmt.Errorf("unknown material %q (have: directional-diffuse, textured)", material)
	}

	mod, layout, err := lower.Lower(m)
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
	glslVert, glslFrag, err := glsl.Emit(mod)
	if err != nil {
		return fmt.Errorf("emit glsl: %w", err)
	}
	desc, err := layout.JSON()
	if err != nil {
		return fmt.Errorf("descriptor: %w", err)
	}

	page := demoHTML
	for ph, val := range map[string]string{
		"__MATERIAL__":   mod.Name,
		"__WGSL__":       strings.TrimSpace(wgslSrc),
		"__GLES_VERT__":  strings.TrimSpace(glesVert),
		"__GLES_FRAG__":  strings.TrimSpace(glesFrag),
		"__GLSL_VERT__":  strings.TrimSpace(glslVert),
		"__GLSL_FRAG__":  strings.TrimSpace(glslFrag),
		"__DESCRIPTOR__": strings.TrimSpace(desc),
	} {
		page = strings.ReplaceAll(page, ph, val)
	}

	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("wrote %s (%s) — open in Chrome (WebGPU needs Chrome 113+)\n", out, mod.Name)
	return nil
}

const demoHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>selena · __MATERIAL__</title>
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
  .ok { color: #79e08a; } .warn { color: #e0c879; } .err { color: #ff7a7a; white-space: pre-wrap; }
  details { padding: 8px 24px 28px; max-width: 760px; }
  summary { cursor: pointer; color: #b9b9cc; }
  pre { background: #07070d; border: 1px solid #1c1c2a; border-radius: 8px;
        padding: 12px; overflow: auto; font-size: 12px; color: #cdd; }
  .note { margin: 0 24px 8px; padding: 10px 12px; border-radius: 8px; max-width: 736px;
          background: #1a1320; border: 1px solid #3a2742; color: #d8c4e6; font-size: 12px; }
</style>
</head>
<body>
<header>
  <h1>🌒 selena · __MATERIAL__</h1>
  <p class="sub">High-level material &rarr; lowered IR &rarr; emitted shaders <b>and</b> a generated
  binding descriptor. Uniforms are packed and textures bound straight from the descriptor.</p>
</header>
<p class="note">If a canvas stays blank: this needs GPU access. On WSL, open the file in
<b>Windows</b> Chrome (not a Linux/WSL Chrome). Check <b>chrome://gpu</b> and enable
<b>Settings &rarr; System &rarr; "Use graphics acceleration when available"</b>. The harness
falls back to software WebGPU and to WebGL1 automatically.</p>

<div class="row">
  <figure>
    <figcaption>WebGPU · WGSL &nbsp;(browser + GoSX desktop)</figcaption>
    <canvas id="gpu" width="360" height="360"></canvas>
    <div id="gpustatus" class="status">starting…</div>
  </figure>
  <figure>
    <figcaption>WebGL · GLSL &nbsp;(browser + Android GLES)</figcaption>
    <canvas id="gl" width="360" height="360"></canvas>
    <div id="glstatus" class="status">starting…</div>
  </figure>
</div>

<details open><summary>generated binding descriptor (drives host packing + texture binding)</summary><pre id="show-desc"></pre></details>
<details><summary>emitted WGSL</summary><pre id="show-wgsl"></pre></details>

<script type="x-shader/wgsl" id="src-wgsl">__WGSL__</script>
<script type="x-shader/glsl" id="src-glesv">__GLES_VERT__</script>
<script type="x-shader/glsl" id="src-glesf">__GLES_FRAG__</script>
<script type="x-shader/glsl" id="src-glslv">__GLSL_VERT__</script>
<script type="x-shader/glsl" id="src-glslf">__GLSL_FRAG__</script>
<script type="application/json" id="src-desc">__DESCRIPTOR__</script>

<script>
"use strict";
function $(id){ return document.getElementById(id); }
function setStatus(id, msg, cls){ var el=$(id); el.textContent=msg; el.className="status "+(cls||""); }

var WGSL = $("src-wgsl").textContent.trim();
var GLES_VERT = $("src-glesv").textContent.trim(), GLES_FRAG = $("src-glesf").textContent.trim();
var GLSL_VERT = $("src-glslv").textContent.trim(), GLSL_FRAG = $("src-glslf").textContent.trim();
var DESC = JSON.parse($("src-desc").textContent);
$("show-desc").textContent = JSON.stringify(DESC, null, 2);
$("show-wgsl").textContent = WGSL;
var HAS_TEX = (DESC.textures || []).length > 0;

var MATERIAL = { baseColor: [0.78, 0.42, 0.98], light_dir: [0.4, 0.85, 0.6], light_ambient: 0.16 };
function uniformValues(sc){ var v = { mvp: sc.mvp, normalMatrix: sc.nrm }; for (var k in MATERIAL) v[k] = MATERIAL[k]; return v; }

function floatCount(t){ return {float:1, vec2:2, vec3:3, vec4:4, mat3:9, mat4:16}[t]; }
function attrFloats(t){ return {vec2:2, vec3:3, vec4:4}[t]; }

// general std140 packer driven by the descriptor (mat3 = 3 cols, 16B stride)
function packUniforms(desc, values){
  var f32 = new Float32Array(desc.uniformBlock.size / 4);
  desc.uniformBlock.fields.forEach(function(f){
    var v = values[f.name], base = f.offset / 4;
    if (f.type === "float") f32[base] = v;
    else if (f.type === "mat3") { for (var c=0;c<3;c++){ f32[base+c*4]=v[c*3]; f32[base+c*4+1]=v[c*3+1]; f32[base+c*4+2]=v[c*3+2]; } }
    else f32.set(v.slice(0, floatCount(f.type)), base);
  });
  return f32;
}

// 8x8 magenta/dark checkerboard, RGBA8
function checker(){
  var n=8, d=new Uint8Array(n*n*4);
  for (var y=0;y<n;y++) for (var x=0;x<n;x++){
    var i=(y*n+x)*4, on=((x^y)&1)===0;
    d[i]=on?210:40; d[i+1]=on?70:20; d[i+2]=on?245:48; d[i+3]=255;
  }
  return { data:d, w:n, h:n };
}

// --- mat helpers + scene ---
function mat4Mul(a,b){ var o=new Array(16); for(var c=0;c<4;c++)for(var r=0;r<4;r++) o[c*4+r]=a[r]*b[c*4]+a[4+r]*b[c*4+1]+a[8+r]*b[c*4+2]+a[12+r]*b[c*4+3]; return o; }
function mat4Perspective(fovy,aspect,near,far){ var f=1/Math.tan(fovy/2),nf=1/(near-far); return [f/aspect,0,0,0, 0,f,0,0, 0,0,(far+near)*nf,-1, 0,0,2*far*near*nf,0]; }
function mat4Translate(x,y,z){ return [1,0,0,0, 0,1,0,0, 0,0,1,0, x,y,z,1]; }
function mat4RotateY(a){ var c=Math.cos(a),s=Math.sin(a); return [c,0,-s,0, 0,1,0,0, s,0,c,0, 0,0,0,1]; }
function mat4RotateX(a){ var c=Math.cos(a),s=Math.sin(a); return [1,0,0,0, 0,c,s,0, 0,-s,c,0, 0,0,0,1]; }
function mat3FromMat4(m){ return [m[0],m[1],m[2], m[4],m[5],m[6], m[8],m[9],m[10]]; }
function scene(t){ var proj=mat4Perspective(Math.PI/4,1,0.1,100), view=mat4Translate(0,0,-4), model=mat4Mul(mat4RotateY(t*0.8),mat4RotateX(t*0.55)); return { mvp: mat4Mul(proj, mat4Mul(view, model)), nrm: mat3FromMat4(model) }; }

// cube: 36 verts, position(3)+normal(3)+uv(2), CCW-outward
function buildCube(){
  var s=0.85;
  var faces=[
    {n:[0,0,1], v:[[-s,-s,s],[s,-s,s],[s,s,s],[-s,s,s]]},
    {n:[0,0,-1],v:[[s,-s,-s],[-s,-s,-s],[-s,s,-s],[s,s,-s]]},
    {n:[1,0,0], v:[[s,-s,s],[s,-s,-s],[s,s,-s],[s,s,s]]},
    {n:[-1,0,0],v:[[-s,-s,-s],[-s,-s,s],[-s,s,s],[-s,s,-s]]},
    {n:[0,1,0], v:[[-s,s,s],[s,s,s],[s,s,-s],[-s,s,-s]]},
    {n:[0,-1,0],v:[[-s,-s,-s],[s,-s,-s],[s,-s,s],[-s,-s,s]]}
  ];
  var uv=[[0,0],[1,0],[1,1],[0,1]], tri=[0,1,2,0,2,3], data=[];
  for(var f=0;f<faces.length;f++) for(var i=0;i<tri.length;i++){ var k=tri[i],p=faces[f].v[k],n=faces[f].n,t=uv[k]; data.push(p[0],p[1],p[2],n[0],n[1],n[2],t[0],t[1]); }
  return new Float32Array(data);
}
var CUBE = buildCube(), VCOUNT = CUBE.length/8, VSTRIDE = 8*4;     // pos3+nrm3+uv2
// byte offset of an attribute name within the interleaved vertex
function attrOffset(name){ return {position:0, normal:12, uv:24}[name]; }

// ============================ WebGPU ============================
async function initWebGPU(){
  if (!navigator.gpu){ setStatus("gpustatus","WebGPU unavailable — needs Chrome 113+ with hardware acceleration","err"); return; }
  var adapter = await navigator.gpu.requestAdapter();
  var soft = false;
  if (!adapter){ adapter = await navigator.gpu.requestAdapter({ forceFallbackAdapter: true }); soft = true; }
  if (!adapter){ setStatus("gpustatus","no WebGPU adapter (HW or software) — see chrome://gpu","err"); return; }
  var device = await adapter.requestDevice();
  if (device.addEventListener) device.addEventListener("uncapturederror", function(e){ setStatus("gpustatus","runtime error: "+e.error.message,"err"); });

  var canvas=$("gpu"), ctx=canvas.getContext("webgpu"), format=navigator.gpu.getPreferredCanvasFormat();
  ctx.configure({ device:device, format:format, alphaMode:"opaque" });

  var vbuf=device.createBuffer({ size:CUBE.byteLength, usage:GPUBufferUsage.VERTEX|GPUBufferUsage.COPY_DST });
  device.queue.writeBuffer(vbuf,0,CUBE);
  var ubuf=device.createBuffer({ size:DESC.uniformBlock.size, usage:GPUBufferUsage.UNIFORM|GPUBufferUsage.COPY_DST });

  var wgslAttrs=[]; DESC.attributes.forEach(function(a){ wgslAttrs.push({ shaderLocation:a.location, offset:attrOffset(a.name), format:"float32x"+attrFloats(a.type) }); });

  var module=device.createShaderModule({ code:WGSL });
  var info=await module.getCompilationInfo();
  for (var i=0;i<info.messages.length;i++) if (info.messages[i].type==="error"){ setStatus("gpustatus","WGSL compile error: "+info.messages[i].message,"err"); return; }

  var pipeline=device.createRenderPipeline({
    layout:"auto",
    vertex:{ module:module, entryPoint:"vertexMain", buffers:[{ arrayStride:VSTRIDE, attributes:wgslAttrs }] },
    fragment:{ module:module, entryPoint:"fragmentMain", targets:[{ format:format }] },
    primitive:{ topology:"triangle-list", cullMode:"back", frontFace:"ccw" }
  });

  var entries=[{ binding:DESC.wgsl.binding, resource:{ buffer:ubuf } }];
  (DESC.textures||[]).forEach(function(t){
    var img=checker();
    var tex=device.createTexture({ size:[img.w,img.h], format:"rgba8unorm", usage:GPUTextureUsage.TEXTURE_BINDING|GPUTextureUsage.COPY_DST });
    device.queue.writeTexture({ texture:tex }, img.data, { bytesPerRow:img.w*4 }, [img.w,img.h]);
    var samp=device.createSampler({ magFilter:"nearest", minFilter:"nearest" });
    entries.push({ binding:t.wgsl.textureBinding, resource:tex.createView() });
    entries.push({ binding:t.wgsl.samplerBinding, resource:samp });
  });
  var bind=device.createBindGroup({ layout:pipeline.getBindGroupLayout(DESC.wgsl.group), entries:entries });

  setStatus("gpustatus","WebGPU OK"+(soft?" (software)":"")+" · packed "+DESC.uniformBlock.size+"B from descriptor", soft?"warn":"ok");
  function frame(now){
    device.queue.writeBuffer(ubuf,0,packUniforms(DESC,uniformValues(scene(now*0.001))));
    var enc=device.createCommandEncoder();
    var pass=enc.beginRenderPass({ colorAttachments:[{ view:ctx.getCurrentTexture().createView(), clearValue:{r:0.03,g:0.03,b:0.05,a:1}, loadOp:"clear", storeOp:"store" }] });
    pass.setPipeline(pipeline); pass.setBindGroup(DESC.wgsl.group,bind); pass.setVertexBuffer(0,vbuf); pass.draw(VCOUNT); pass.end();
    device.queue.submit([enc.finish()]); requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

// ============================ WebGL (2, then 1) ============================
function initWebGL(){
  var canvas=$("gl"), gl=canvas.getContext("webgl2"), es3=true;
  if (!gl){ gl=canvas.getContext("webgl"); es3=false; }
  if (!gl){ setStatus("glstatus","WebGL unavailable — see chrome://gpu / enable hardware acceleration","err"); return; }
  var vsrc = es3?GLES_VERT:GLSL_VERT, fsrc = es3?GLES_FRAG:GLSL_FRAG;

  function compile(type,src,label){ var s=gl.createShader(type); gl.shaderSource(s,src); gl.compileShader(s); if(!gl.getShaderParameter(s,gl.COMPILE_STATUS)){ setStatus("glstatus",label+" error: "+gl.getShaderInfoLog(s),"err"); return null; } return s; }
  var vs=compile(gl.VERTEX_SHADER,vsrc,"vertex"), fs=compile(gl.FRAGMENT_SHADER,fsrc,"fragment"); if(!vs||!fs) return;
  var prog=gl.createProgram(); gl.attachShader(prog,vs); gl.attachShader(prog,fs); gl.linkProgram(prog);
  if(!gl.getProgramParameter(prog,gl.LINK_STATUS)){ setStatus("glstatus","link error: "+gl.getProgramInfoLog(prog),"err"); return; }

  var vbo=gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER,vbo); gl.bufferData(gl.ARRAY_BUFFER,CUBE,gl.STATIC_DRAW);
  var ptrs=[]; DESC.attributes.forEach(function(a){ ptrs.push({ loc:gl.getAttribLocation(prog,a.name), n:attrFloats(a.type), offset:attrOffset(a.name) }); });
  var setters={ float:"uniform1f", vec2:"uniform2fv", vec3:"uniform3fv", vec4:"uniform4fv" };

  gl.useProgram(prog);
  (DESC.textures||[]).forEach(function(t){
    var img=checker(), tex=gl.createTexture();
    gl.activeTexture(gl.TEXTURE0+t.gl.unit); gl.bindTexture(gl.TEXTURE_2D,tex);
    gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,img.w,img.h,0,gl.RGBA,gl.UNSIGNED_BYTE,img.data);
    gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.NEAREST); gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE); gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);
    gl.uniform1i(gl.getUniformLocation(prog,t.gl.uniform), t.gl.unit);
  });
  gl.enable(gl.CULL_FACE); gl.cullFace(gl.BACK); gl.frontFace(gl.CCW); gl.clearColor(0.03,0.03,0.05,1);
  setStatus("glstatus", (es3?"WebGL2":"WebGL1")+" OK · "+gl.getParameter(gl.VERSION), es3?"ok":"warn");

  function frame(now){
    var vals=uniformValues(scene(now*0.001));
    gl.viewport(0,0,canvas.width,canvas.height); gl.clear(gl.COLOR_BUFFER_BIT); gl.useProgram(prog);
    DESC.uniformBlock.fields.forEach(function(f){
      var loc=gl.getUniformLocation(prog,f.name), v=vals[f.name];
      if (f.type==="mat3") gl.uniformMatrix3fv(loc,false,v);
      else if (f.type==="mat4") gl.uniformMatrix4fv(loc,false,v);
      else gl[setters[f.type]](loc,v);
    });
    gl.bindBuffer(gl.ARRAY_BUFFER,vbo);
    ptrs.forEach(function(p){ gl.enableVertexAttribArray(p.loc); gl.vertexAttribPointer(p.loc,p.n,gl.FLOAT,false,VSTRIDE,p.offset); });
    gl.drawArrays(gl.TRIANGLES,0,VCOUNT); requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

initWebGPU().catch(function(e){ setStatus("gpustatus","WebGPU init failed: "+e.message,"err"); });
initWebGL();
</script>
</body>
</html>
`
