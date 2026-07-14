package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.WGSL{}

// Emit renders m as a single WGSL source string exposing vertexMain and
// fragmentMain, for the WebGPU backend (browser + GoSX desktop via Chromium).
func Emit(m ir.Module) (string, error) {
	switch m.Kind {
	case ir.KindPoints:
		return emitPoints(m)
	case ir.KindPost:
		return emitPost(m)
	case ir.KindFeedback:
		return emitFeedback(m)
	default:
		return emitMesh(m)
	}
}

// emitFeedback emits a WGSL compute kernel for a feedback-kind simulation step.
//
// One @compute @workgroup_size(64) entry computes the cell index from the
// invocation id (guarded by gridLen), evaluates the feedback body — where each
// state(dx, dy) read lowers to an inState[...] gather — and writes the result
// to outState[cellIndex].
//
// Binding contract (@group(0)):
//
//	@binding(0)  GridUniforms { gridWidth, gridLen }
//	@binding(1)  inState  : array<vec4<f32>>  (storage, read)      — previous state
//	@binding(2)  outState : array<vec4<f32>>  (storage, read_write) — next state
//	@binding(3)  UserUniforms (from m.Uniforms, if any)
//
// The host ping-pongs the two storage buffers each step (swap in/out).
func emitFeedback(m ir.Module) (string, error) {
	var b strings.Builder

	// Kind-injected grid uniforms + the statefield ping-pong buffers.
	b.WriteString("struct GridUniforms {\n  gridWidth : u32,\n  gridLen   : u32,\n};\n")
	b.WriteString("@group(0) @binding(0) var<uniform> _grid : GridUniforms;\n")
	b.WriteString("@group(0) @binding(1) var<storage, read> inState : array<vec4<f32>>;\n")
	b.WriteString("@group(0) @binding(2) var<storage, read_write> outState : array<vec4<f32>>;\n")

	// User uniforms at @group(0) @binding(3).
	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("\nstruct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(0) @binding(3) var<uniform> u : UserUniforms;\n")
	}

	allUniforms := internal.NameSet(m.Uniforms)
	for _, au := range m.ArrayUniforms {
		allUniforms[au.Name] = true
	}
	fs := internal.Resolver{
		Dialect:   prismDialect,
		Uniforms:  allUniforms,
		Fragment:  true,
		Qualified: true,
		StateSampleFn: func(dx, dy int64) string {
			return fmt.Sprintf(
				"inState[clamp(i32(cellIndex) + (%d) + (%d) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)]",
				dx, dy)
		},
		CellUVFn: func() string {
			return "(vec2<f32>(f32(cellIndex % _grid.gridWidth), f32(cellIndex / _grid.gridWidth)) + vec2<f32>(0.5, 0.5)) / f32(_grid.gridWidth)"
		},
	}

	b.WriteString("\n@compute @workgroup_size(64)\nfn computeMain(@builtin(global_invocation_id) gid : vec3<u32>) {\n")
	b.WriteString("  let cellIndex = gid.x;\n")
	b.WriteString("  if (cellIndex >= _grid.gridLen) { return; }\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  outState[cellIndex] = %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// emitMesh is the original mesh pipeline emitter.
func emitMesh(m ir.Module) (string, error) {
	if m.VertexAuthored {
		return emitMeshAuthored(m)
	}
	var b strings.Builder

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(0) @binding(0) var<uniform> u : Uniforms;\n\n")
	}

	// Textures: WGSL binds the texture and its sampler separately. Convention:
	// uniform block is binding 0, then texture i takes 1+2i and its sampler 2+2i.
	// Cube-map textures use texture_cube<f32>; regular 2D textures use texture_2d<f32>.
	for i, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_cube<f32>;\n", 1+2*i, t.Name)
		} else {
			fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, t.Name)
		}
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %sSampler : sampler;\n", 2+2*i, t.Name)
	}
	if len(m.Textures) > 0 {
		b.WriteString("\n")
	}

	b.WriteString("struct VertexInput {\n")
	for i, a := range m.Attributes {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, a.Name, typeName(a.Type))
	}
	b.WriteString("};\n\n")

	b.WriteString("struct VertexOutput {\n  @builtin(position) position : vec4<f32>,\n")
	for i, v := range m.Varyings {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, v.Name, typeName(v.Type))
	}
	b.WriteString("};\n\n")

	// Vertex stage.
	vs := internal.NewQualified(prismDialect, m, false)
	b.WriteString("@vertex\nfn vertexMain(in : VertexInput) -> VertexOutput {\n  var out : VertexOutput;\n")
	for _, s := range m.Vertex.Body {
		if vs.Varyings[s.Target] {
			fmt.Fprintf(&b, "  out.%s = %s;\n", s.Target, ir.Print(s.Value, vs))
		} else {
			fmt.Fprintf(&b, "  let %s = %s;\n", s.Target, ir.Print(s.Value, vs))
		}
	}
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	// Fragment stage.
	fs := internal.NewQualified(prismDialect, m, true)
	b.WriteString("@fragment\nfn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// emitMeshAuthored emits a mesh material that authors its own vertex() stage (B4).
//
// The vertex entry takes its index source from @builtin(vertex_index) (procedural
// geometry) and/or per-vertex attributes, computes the clip-space position from
// the author body, and writes author-declared @location varyings into
// VertexOutput. The fragment entry reads those varyings. An optional statefield
// adds a read-only inState storage buffer + a StateGrid uniform addressed by
// stateAt(uv). Legacy mesh materials go through emitMesh and are byte-identical.
func emitMeshAuthored(m ir.Module) (string, error) {
	var b strings.Builder

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(0) @binding(0) var<uniform> u : Uniforms;\n\n")
	}

	for i, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_cube<f32>;\n", 1+2*i, t.Name)
		} else {
			fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, t.Name)
		}
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %sSampler : sampler;\n", 2+2*i, t.Name)
	}
	if len(m.Textures) > 0 {
		b.WriteString("\n")
	}

	// Optional statefield read bindings (stateAt(uv)).
	if m.StateField != "" {
		base := 1 + 2*len(m.Textures)
		b.WriteString("struct StateGrid {\n  gridWidth  : u32,\n  gridHeight : u32,\n};\n")
		fmt.Fprintf(&b, "@group(0) @binding(%d) var<uniform> _stateGrid : StateGrid;\n", base)
		fmt.Fprintf(&b, "@group(0) @binding(%d) var _inState : texture_2d<f32>;\n\n", base+1)
	}

	// VertexInput only when attributes are actually read (procedural geometry has none).
	if len(m.Attributes) > 0 {
		b.WriteString("struct VertexInput {\n")
		for i, a := range m.Attributes {
			fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, a.Name, typeName(a.Type))
		}
		b.WriteString("};\n\n")
	}

	b.WriteString("struct VertexOutput {\n  @builtin(position) position : vec4<f32>,\n")
	for i, v := range m.Varyings {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, v.Name, typeName(v.Type))
	}
	b.WriteString("};\n\n")

	// Vertex stage.
	vs := internal.NewQualified(prismDialect, m, false)
	vs.StateSampleUVFn = wgslStateSampleUV
	var vparams []string
	if m.UsesVertexIndex {
		vparams = append(vparams, "@builtin(vertex_index) vertexIndex : u32")
	}
	if len(m.Attributes) > 0 {
		vparams = append(vparams, "in : VertexInput")
	}
	fmt.Fprintf(&b, "@vertex\nfn vertexMain(%s) -> VertexOutput {\n  var out : VertexOutput;\n", strings.Join(vparams, ", "))
	internal.EmitStmtList(&b, m.Vertex.Body, vs, "  ", true)
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	// Fragment stage.
	fs := internal.NewQualified(prismDialect, m, true)
	fs.StateSampleUVFn = wgslStateSampleUV
	b.WriteString("@fragment\nfn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// wgslStateSampleUV renders a stateAt(uv) read against the inState texture, addressing
// the grid by uv -> integer texel using the StateGrid dims.
//
// textureLoad, not textureSample: it takes integer coordinates and needs no sampler, so
// it selects exactly the texel the previous flat-index read selected (same truncation)
// and is bit-identical, while going through the texture cache rather than the raw buffer
// path. It also carries no implicit derivative, so it stays legal in the vertex stage,
// where a mesh material's stateAt() displaces geometry. The clamp is now per-axis, which
// additionally fixes a wrap: clamping a FLAT index let a uv slightly past 1.0 in x bleed
// into the first texel of the next ROW, where WebGL's CLAMP_TO_EDGE holds the edge. B4.
func wgslStateSampleUV(uv string) string {
	return fmt.Sprintf(
		"textureLoad(_inState, vec2<u32>(min(u32((%s).x * f32(_stateGrid.gridWidth)), _stateGrid.gridWidth - 1u), min(u32((%s).y * f32(_stateGrid.gridHeight)), _stateGrid.gridHeight - 1u)), 0)",
		uv, uv)
}

// emitPoints emits a WGSL billboard quad points/particle surface.
//
// The module contains TWO vertex entry points matching the two gosx pipelines:
//
//	vertexMain        — reads vertex attributes (WGSL_POINTS_INSTANCED_VERTEX):
//	                    @location(0) position f32x3, @location(1) size f32,
//	                    @location(2) color f32x4; stride 32, per-instance.
//	                    Groups: @group(0)@binding(0) FrameUniforms,
//	                            @group(2)@binding(0) PointsUniforms,
//	                            @group(1)@binding(0) UserUniforms (if any).
//	                    Draw: drawIndexed(6, instanceCount).
//
//	vertexStorageMain — reads from @group(2)@binding(1) storage buffer (WGSL_POINTS_VERTEX):
//	                    same groups PLUS the storage binding.
//	                    Draw: draw(6, instanceCount) with @builtin(instance_index).
//
// Both share the same PointsOutput/PointsInput structs and fragmentMain.
// WebGPU validates bindings per entry point, so the storage declaration is
// legal in the module even when vertexMain doesn't reference it.
//
// Binding contract:
//
//	@group(0) @binding(0)  FrameUniforms
//	@group(2) @binding(0)  PointsUniforms
//	@group(2) @binding(1)  array<ParticleInstance>  (only used by vertexStorageMain)
//	@group(1) @binding(0)  UserUniforms (from m.Uniforms, if any)
func emitPoints(m ir.Module) (string, error) {
	var b strings.Builder

	// Engine structs.
	b.WriteString(`struct FrameUniforms {
  viewMatrix     : mat4x4<f32>,
  projMatrix     : mat4x4<f32>,
  viewportWidth  : f32,
  viewportHeight : f32,
};

struct PointsUniforms {
  modelMatrix        : mat4x4<f32>,
  defaultColorAndSize: vec4<f32>,
  flags              : vec4<u32>,
  params             : vec4<f32>,
  fogColor           : vec4<f32>,
};

struct ParticleInstance {
  position : vec3<f32>,
  size     : f32,
  color    : vec4<f32>,
};

@group(0) @binding(0) var<uniform> _frame     : FrameUniforms;
@group(2) @binding(0) var<uniform> _points    : PointsUniforms;
@group(2) @binding(1) var<storage, read> _particles : array<ParticleInstance>;
`)

	// User uniforms at @group(1) @binding(0).
	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("\nstruct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(1) @binding(0) var<uniform> u : UserUniforms;\n")
	}

	b.WriteString(`
struct PointsOutput {
  @builtin(position) clipPos : vec4<f32>,
  @location(0) v_color       : vec3<f32>,
  @location(1) v_fogFactor   : f32,
  @location(2) v_alpha       : f32,
  @location(3) v_pointCoord  : vec2<f32>,
  @location(4) v_pointSize   : f32,
};

// Attribute input for the static-layer vertex entry (vertexMain).
struct PointsAttribInput {
  @location(0) a_position : vec3<f32>,
  @location(1) a_size     : f32,
  @location(2) a_color    : vec4<f32>,
};

const _quadPos = array<vec2<f32>, 6>(
  vec2<f32>(-0.5, -0.5), vec2<f32>(0.5, -0.5), vec2<f32>(-0.5, 0.5),
  vec2<f32>(0.5, -0.5),  vec2<f32>(0.5, 0.5),  vec2<f32>(-0.5, 0.5),
);

`)

	// _pointsQuadExpand is a shared inline helper comment; the actual math is
	// duplicated in each entry because WGSL has no plain function overloading
	// across different data sources cleanly — but both compile identically.

	// ---- vertexMain: attribute variant (WGSL_POINTS_INSTANCED_VERTEX) ----
	b.WriteString(`@vertex fn vertexMain(
  @builtin(vertex_index) vertexIndex : u32,
  in : PointsAttribInput,
) -> PointsOutput {
  let quad = _quadPos[vertexIndex];

  let worldPos = (_points.modelMatrix * vec4<f32>(in.a_position, 1.0)).xyz;
  let viewPos  = _frame.viewMatrix * vec4<f32>(worldPos, 1.0);

  var rawSize : f32;
  if (_points.flags.y == 0u) { rawSize = _points.defaultColorAndSize.w; }
  else { rawSize = in.a_size; }

  var pixelSize : f32;
  if (_points.flags.z != 0u) {
    pixelSize = max(rawSize * (_frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  let _minPx = max(_points.fogColor.a, 0.0);
  if (_minPx > 0.0) { pixelSize = max(pixelSize, _minPx); }
  if (_points.params.w > 0.0) { pixelSize = min(pixelSize, _points.params.w); }

  let clipPos    = _frame.projMatrix * viewPos;
  let ndcOffsetX = quad.x * pixelSize / _frame.viewportWidth  * clipPos.w * 2.0;
  let ndcOffsetY = quad.y * pixelSize / _frame.viewportHeight * clipPos.w * 2.0;

  var out : PointsOutput;
  out.clipPos = vec4<f32>(clipPos.x + ndcOffsetX, clipPos.y + ndcOffsetY, clipPos.z, clipPos.w);

  if (_points.flags.x != 0u) { out.v_color = in.a_color.rgb; }
  else { out.v_color = _points.defaultColorAndSize.rgb; }
  out.v_alpha      = in.a_color.a * _points.params.x;
  out.v_pointCoord = quad + vec2<f32>(0.5, 0.5);
  out.v_pointSize  = pixelSize;

  if (_points.params.y != 0.0) {
    let dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-_points.params.z * _points.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

`)

	// ---- vertexStorageMain: storage/particle variant (WGSL_POINTS_VERTEX) ----
	b.WriteString(`@vertex fn vertexStorageMain(
  @builtin(vertex_index)   vertexIndex   : u32,
  @builtin(instance_index) instanceIndex : u32,
) -> PointsOutput {
  let quad = _quadPos[vertexIndex];
  let p    = _particles[instanceIndex];

  let worldPos = (_points.modelMatrix * vec4<f32>(p.position, 1.0)).xyz;
  let viewPos  = _frame.viewMatrix * vec4<f32>(worldPos, 1.0);

  var rawSize : f32;
  if (_points.flags.y == 0u) { rawSize = _points.defaultColorAndSize.w; }
  else { rawSize = p.size; }

  var pixelSize : f32;
  if (_points.flags.z != 0u) {
    pixelSize = max(rawSize * (_frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  let _minPx = max(_points.fogColor.a, 0.0);
  if (_minPx > 0.0) { pixelSize = max(pixelSize, _minPx); }
  if (_points.params.w > 0.0) { pixelSize = min(pixelSize, _points.params.w); }

  let clipPos    = _frame.projMatrix * viewPos;
  let ndcOffsetX = quad.x * pixelSize / _frame.viewportWidth  * clipPos.w * 2.0;
  let ndcOffsetY = quad.y * pixelSize / _frame.viewportHeight * clipPos.w * 2.0;

  var out : PointsOutput;
  out.clipPos = vec4<f32>(clipPos.x + ndcOffsetX, clipPos.y + ndcOffsetY, clipPos.z, clipPos.w);

  if (_points.flags.x != 0u) { out.v_color = p.color.rgb; }
  else { out.v_color = _points.defaultColorAndSize.rgb; }
  out.v_alpha      = p.color.a * _points.params.x;
  out.v_pointCoord = quad + vec2<f32>(0.5, 0.5);
  out.v_pointSize  = pixelSize;

  if (_points.params.y != 0.0) {
    let dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-_points.params.z * _points.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

`)

	// Fragment stage: author body + output.
	b.WriteString("struct PointsInput {\n")
	b.WriteString("  @location(0) v_color      : vec3<f32>,\n")
	b.WriteString("  @location(1) v_fogFactor  : f32,\n")
	b.WriteString("  @location(2) v_alpha      : f32,\n")
	b.WriteString("  @location(3) v_pointCoord : vec2<f32>,\n")
	b.WriteString("  @location(4) v_pointSize  : f32,\n")
	b.WriteString("};\n\n")

	// Build the fragment resolver. Varyings come in via the PointsInput struct.
	varyingSet := map[string]bool{
		"v_color": true, "v_fogFactor": true, "v_alpha": true,
		"v_pointCoord": true, "v_pointSize": true,
	}
	fs := internal.Resolver{
		Dialect:   prismDialect,
		Uniforms:  internal.NameSet(m.Uniforms),
		Varyings:  varyingSet,
		Fragment:  true,
		Qualified: true,
	}

	b.WriteString("@fragment fn fragmentMain(in : PointsInput) -> @location(0) vec4<f32> {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// emitPost emits a WGSL fullscreen post-process pass surface.
//
// Binding contract:
//
//	@group(0) @binding(0)  sceneColor texture_2d<f32>  (engine)
//	@group(0) @binding(1)  sceneColor sampler           (engine)
//	@group(0) @binding(2)  sceneDepth texture_depth_2d  (engine)
//	@group(0) @binding(3)  sceneDepth sampler           (engine)
//	@group(0) @binding(4)  UserUniforms (if any)
//
// The vertex stage is a fullscreen triangle (no attributes).
// The fragment stage body is the author surface, with uv plumbed in.
func emitPost(m ir.Module) (string, error) {
	var b strings.Builder

	// Engine-provided scene textures at fixed slots.
	b.WriteString(`@group(0) @binding(0) var _sceneColorTex  : texture_2d<f32>;
@group(0) @binding(1) var _sceneColorSamp : sampler;
@group(0) @binding(2) var _sceneDepthTex  : texture_depth_2d;
@group(0) @binding(3) var _sceneDepthSamp : sampler;
`)

	// User uniforms at @group(0) @binding(4).
	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("\nstruct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(0) @binding(4) var<uniform> u : UserUniforms;\n")
	}

	// Fullscreen triangle vertex stage (no attributes needed).
	b.WriteString(`
struct PostOutput {
  @builtin(position) pos : vec4<f32>,
  @location(0) v_uv      : vec2<f32>,
};

const _postPositions = array<vec2<f32>, 3>(
  vec2<f32>(-1.0, -1.0),
  vec2<f32>( 3.0, -1.0),
  vec2<f32>(-1.0,  3.0),
);
const _postUVs = array<vec2<f32>, 3>(
  vec2<f32>(0.0, 1.0),
  vec2<f32>(2.0, 1.0),
  vec2<f32>(0.0, -1.0),
);

@vertex fn vertexMain(@builtin(vertex_index) vi : u32) -> PostOutput {
  var out : PostOutput;
  out.pos  = vec4<f32>(_postPositions[vi], 0.0, 1.0);
  out.v_uv = _postUVs[vi];
  return out;
}

`)

	// Fragment input struct.
	b.WriteString("struct PostInput {\n  @location(0) v_uv : vec2<f32>,\n};\n\n")

	// Build resolver. Varyings = v_uv. SceneSampleFn renders scene samples.
	// Array uniforms must join the uniform name set so refs like spheres[i]
	// resolve through the u. struct accessor (u.spheres[i]).
	varyingSet := map[string]bool{"v_uv": true}
	uniformSet := internal.NameSet(m.Uniforms)
	for _, au := range m.ArrayUniforms {
		uniformSet[au.Name] = true
	}
	fs := internal.Resolver{
		Dialect:   prismDialect,
		Uniforms:  uniformSet,
		Varyings:  varyingSet,
		Fragment:  true,
		Qualified: true,
		SceneSampleFn: func(name, uv string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("textureSample(_sceneColorTex, _sceneColorSamp, %s)", uv)
			case "sceneDepth":
				return fmt.Sprintf("textureSampleCompare(_sceneDepthTex, _sceneDepthSamp, %s, 0.0)", uv)
			default:
				return "vec4<f32>(0.0)"
			}
		},
	}

	b.WriteString("@fragment fn fragmentMain(in : PostInput) -> @location(0) vec4<f32> {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// typeName spells an ir.Type in WGSL, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
