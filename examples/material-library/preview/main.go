// Command preview emits a library material and a WebGPU/WebGL comparison page.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"m31labs.dev/selena"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/materiallib"
)

func main() {
	sourcePath := flag.String("source", "examples/material-library/brdf.sel", "material source")
	moduleList := flag.String("modules", "brdf", "comma-separated library modules")
	output := flag.String("out", "material-preview.html", "comparison HTML")
	artifacts := flag.String("artifacts", "", "optional directory for emitted shaders and descriptor")
	flag.Parse()
	if err := run(*sourcePath, *moduleList, *output, *artifacts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(sourcePath, moduleList, output, artifacts string) error {
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	var modules []materiallib.Module
	for _, m := range strings.Split(moduleList, ",") {
		modules = append(modules, materiallib.Module(m))
	}
	result, err := materiallib.Compile(source, selena.CompileOptions{}, modules...)
	if err != nil {
		return err
	}
	values := map[string]any{
		"mvp":          []float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1},
		"normalMatrix": []float32{1, 0, 0, 0, 1, 0, 0, 0, 1},
	}
	for _, d := range result.Layout.UniformBlock.Defaults {
		if len(d.Values) == 1 {
			values[d.Name] = d.Values[0]
		} else {
			values[d.Name] = d.Values
		}
	}
	packed, err := bindings.PackUniformsWithDefaults(result.Layout, values)
	if err != nil {
		return err
	}
	bytes := make([]int, len(packed))
	for i, b := range packed {
		bytes[i] = int(b)
	}
	wgsl, _ := result.Artifact(selena.TargetWGSL)
	glsl, _ := result.Artifact(selena.TargetGLSL)
	gles, _ := result.Artifact(selena.TargetGLES)
	data, err := json.Marshal(map[string]any{"layout": result.Layout, "values": values, "packed": bytes, "wgsl": wgsl.Source, "glsl": glsl, "gles": gles})
	if err != nil {
		return err
	}
	page := strings.ReplaceAll(previewHTML, "__DATA__", string(data))
	if err := os.WriteFile(output, []byte(page), 0644); err != nil {
		return err
	}
	if artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			return err
		}
		for _, a := range result.Artifacts {
			files := map[string]string{}
			if a.Source != "" {
				files[string(a.Target)] = a.Source
			} else {
				files[string(a.Target)+".vert"] = a.Vertex
				files[string(a.Target)+".frag"] = a.Fragment
			}
			for suffix, content := range files {
				if err := os.WriteFile(filepath.Join(artifacts, result.Module.Name+"."+suffix), []byte(content), 0644); err != nil {
					return err
				}
			}
		}
		descriptor, _ := result.Layout.JSON()
		if err := os.WriteFile(filepath.Join(artifacts, result.Module.Name+".json"), []byte(descriptor), 0644); err != nil {
			return err
		}
	}
	for _, a := range result.Artifacts {
		fmt.Printf("%s %s: %d bytes\n", result.Module.Name, a.Target, len(a.Source)+len(a.Vertex)+len(a.Fragment))
	}
	return nil
}

const previewHTML = `<!doctype html><html lang="en"><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Selena material comparison</title>
<style>body{background:#121719;color:#e7e7dd;font:16px system-ui;margin:24px}h1{font-size:24px}main{display:grid;gap:24px;max-width:960px}canvas{width:100%;height:auto;display:block;background:#050607}figure{margin:0}figcaption{padding:8px 0}.status{font-size:13px}input{width:min(400px,70vw)}</style>
<h1 id="title">Selena material comparison</h1><p>One material, two browser shader targets. Drag the age control for event previews.</p>
<label>Event age (seconds) <input id="age" type="range" min="-0.1" max="1.2" step="0.01" value="0.15"><output id="ageValue">0.15</output></label>
<main><figure><figcaption>WebGPU · WGSL</figcaption><canvas id="gpu" width="900" height="360"></canvas><p class="status" id="gpuStatus">Starting</p></figure>
<figure><figcaption>WebGL · GLSL ES</figcaption><canvas id="gl" width="900" height="360"></canvas><p class="status" id="glStatus">Starting</p></figure></main>
<script id="data" type="application/json">__DATA__</script><script>
const data=JSON.parse(document.getElementById('data').textContent), layout=data.layout;
document.getElementById('title').textContent=layout.material;
if(data.values.flyToHoist>0)document.querySelectorAll('canvas').forEach(c=>c.height=Math.round(c.width/(3*data.values.flyToHoist)));
const arrays={position:new Float32Array([-1,-1,0,1,-1,0,-1,1,0,-1,1,0,1,-1,0,1,1,0]),normal:new Float32Array(Array(6).fill([0,0,1]).flat()),uv:new Float32Array([0,0,1,0,0,1,0,1,1,0,1,1])};
const identity=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1];
let drawGPU=()=>{},drawGL=()=>{},gpuBytes=new Uint8Array(data.packed);
const status=(id,text)=>document.getElementById(id).textContent=text;
function startGL(){
 const canvas=document.getElementById('gl'), gl=canvas.getContext('webgl2',{preserveDrawingBuffer:true})||canvas.getContext('webgl',{preserveDrawingBuffer:true}); if(!gl)throw Error('WebGL unavailable');
 const modern=gl instanceof WebGL2RenderingContext, artifact=modern?data.gles:data.glsl;
 for(const extension of layout.requires?.glExtensions||[])gl.getExtension(extension);
 const compile=(kind,source)=>{const shader=gl.createShader(kind);gl.shaderSource(shader,source);gl.compileShader(shader);if(!gl.getShaderParameter(shader,gl.COMPILE_STATUS))throw Error(gl.getShaderInfoLog(shader));return shader};
 const program=gl.createProgram();gl.attachShader(program,compile(gl.VERTEX_SHADER,artifact.Vertex));gl.attachShader(program,compile(gl.FRAGMENT_SHADER,artifact.Fragment));gl.linkProgram(program);if(!gl.getProgramParameter(program,gl.LINK_STATUS))throw Error(gl.getProgramInfoLog(program));gl.useProgram(program);
 for(const attribute of layout.attributes){const location=gl.getAttribLocation(program,attribute.name);if(location<0)continue;const buffer=gl.createBuffer();gl.bindBuffer(gl.ARRAY_BUFFER,buffer);gl.bufferData(gl.ARRAY_BUFFER,arrays[attribute.name],gl.STATIC_DRAW);gl.enableVertexAttribArray(location);gl.vertexAttribPointer(location,attribute.type==='vec2'?2:3,gl.FLOAT,false,0,0)}
 const bind=()=>{for(const field of layout.uniformBlock.fields){const location=gl.getUniformLocation(program,field.name);const v=data.values[field.name];if(location===null||v===undefined)continue;if(field.type==='mat4')gl.uniformMatrix4fv(location,false,v);else if(field.type==='mat3')gl.uniformMatrix3fv(location,false,v);else if(field.type==='float')gl.uniform1f(location,v);else gl['uniform'+field.type.slice(3)+'fv'](location,v)}};
 drawGL=()=>{bind();gl.viewport(0,0,canvas.width,canvas.height);gl.drawArrays(gl.TRIANGLES,0,6);if(gl.getError()!==gl.NO_ERROR)throw Error('WebGL draw error')};drawGL();status('glStatus',modern?'Rendered with WebGL2':'Rendered with WebGL1');
}
async function startGPU(){
 if(!navigator.gpu)throw Error('WebGPU unavailable');const adapter=await navigator.gpu.requestAdapter();if(!adapter)throw Error('WebGPU adapter unavailable');const device=await adapter.requestDevice();device.addEventListener('uncapturederror',e=>status('gpuStatus',e.error.message));
 const canvas=document.getElementById('gpu'),context=canvas.getContext('webgpu'),format=navigator.gpu.getPreferredCanvasFormat();context.configure({device,format,alphaMode:'opaque'});
 const module=device.createShaderModule({code:data.wgsl});const info=await module.getCompilationInfo();const errors=info.messages.filter(m=>m.type==='error');if(errors.length)throw Error(errors.map(m=>m.message).join('\n'));
 const buffers=layout.attributes.map(a=>({arrayStride:a.type==='vec2'?8:12,attributes:[{shaderLocation:a.location,offset:0,format:a.type==='vec2'?'float32x2':'float32x3'}]}));
 const pipeline=await device.createRenderPipelineAsync({layout:'auto',vertex:{module,entryPoint:layout.entryPoints.vertex,buffers},fragment:{module,entryPoint:layout.entryPoints.fragment,targets:[{format}]},primitive:{topology:'triangle-list'}});
 const uniform=device.createBuffer({size:gpuBytes.length,usage:GPUBufferUsage.UNIFORM|GPUBufferUsage.COPY_DST});const group=device.createBindGroup({layout:pipeline.getBindGroupLayout(layout.wgsl.group),entries:[{binding:layout.wgsl.binding,resource:{buffer:uniform}}]});
 const vertexBuffers=layout.attributes.map(a=>{const array=arrays[a.name],buffer=device.createBuffer({size:array.byteLength,usage:GPUBufferUsage.VERTEX|GPUBufferUsage.COPY_DST});device.queue.writeBuffer(buffer,0,array);return buffer});
 drawGPU=()=>{device.queue.writeBuffer(uniform,0,gpuBytes);const encoder=device.createCommandEncoder(),pass=encoder.beginRenderPass({colorAttachments:[{view:context.getCurrentTexture().createView(),loadOp:'clear',storeOp:'store',clearValue:{r:0,g:0,b:0,a:1}}]});pass.setPipeline(pipeline);pass.setBindGroup(layout.wgsl.group,group);vertexBuffers.forEach((b,i)=>pass.setVertexBuffer(i,b));pass.draw(6);pass.end();device.queue.submit([encoder.finish()])};drawGPU();await device.queue.onSubmittedWorkDone();status('gpuStatus','Rendered with WebGPU');
}
document.getElementById('age').addEventListener('input',e=>{const age=Number(e.target.value);document.getElementById('ageValue').textContent=age.toFixed(2);data.values.age=age;const field=layout.uniformBlock.fields.find(f=>f.name==='age');if(field)new DataView(gpuBytes.buffer).setFloat32(field.offset,age,true);drawGL();drawGPU()});
try{startGL()}catch(e){status('glStatus',e.message)}startGPU().catch(e=>status('gpuStatus',e.message));
</script></html>`
