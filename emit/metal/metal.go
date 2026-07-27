package metal

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.Metal{}

// Emit renders m as a single Metal Shading Language source with vertexMain and
// fragmentMain, for the iOS SceneKit backend consumed by gosx-native. The
// internal.Recover wrapper converts a structured emission failure
// (ir.EmitError, raised by ir.Print or emit/internal's shared statement
// emitter — see their doc comments) into this function's normal error return.
func Emit(m ir.Module) (string, error) {
	return internal.Recover(func() (string, error) {
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
	})
}

// emitFeedback emits a Metal compute kernel for a feedback-kind simulation step,
// analogous to the WGSL compute path (parity). state(dx, dy) reads gather from
// the inState buffer; the result is written to outState[cellIndex].
//
// Buffer contract: inState [[buffer(0)]], outState [[buffer(1)]],
// GridUniforms [[buffer(2)]], UserUniforms [[buffer(3)]] (if any).
func emitFeedback(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")
	b.WriteString("struct GridUniforms {\n  uint gridWidth;\n  uint gridLen;\n};\n\n")

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
		}
		b.WriteString("};\n\n")
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
				"inState[clamp(int(cellIndex) + (%d) + (%d) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)]",
				dx, dy)
		},
		CellUVFn: func() string {
			return "(float2(float(cellIndex % _grid.gridWidth), float(cellIndex / _grid.gridWidth)) + float2(0.5, 0.5)) / float(_grid.gridWidth)"
		},
		// computeMain returns void — an early ir.ReturnCF writes outState then
		// bare-returns, matching the final unconditional write below it.
		ReturnFn: func(val string) string {
			return "outState[cellIndex] = " + val + "; return;"
		},
	}

	b.WriteString("kernel void computeMain(\n")
	b.WriteString("  uint3 gid [[thread_position_in_grid]],\n")
	b.WriteString("  const device float4* inState [[buffer(0)]],\n")
	b.WriteString("  device float4* outState [[buffer(1)]],\n")
	b.WriteString("  constant GridUniforms& _grid [[buffer(2)]]")
	if len(m.Uniforms) > 0 {
		b.WriteString(",\n  constant UserUniforms& u [[buffer(3)]]")
	}
	b.WriteString("\n) {\n")
	b.WriteString("  uint cellIndex = gid.x;\n")
	b.WriteString("  if (cellIndex >= _grid.gridLen) { return; }\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  outState[cellIndex] = %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}

// emitMesh is the original mesh pipeline emitter.
// NOTE: this emits standalone MSL functions. The SceneKit binding strategy
// (SCNProgram semantic uniforms vs. a packed argument buffer) is the binding-
// model design decision the slice is meant to expose; for now uniforms are a
// single constant buffer at [[buffer(0)]].
func emitMesh(m ir.Module) (string, error) {
	if m.VertexAuthored {
		return emitMeshAuthored(m)
	}
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString("struct VertexIn {\n")
	for i, a := range m.Attributes {
		fmt.Fprintf(&b, "  %s %s [[attribute(%d)]];\n", typeName(a.Type), a.Name, i)
	}
	b.WriteString("};\n\n")

	b.WriteString("struct VertexOut {\n  float4 position [[position]];\n")
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "  %s %s;\n", typeName(v.Type), v.Name)
	}
	b.WriteString("};\n\n")

	vs := internal.NewQualified(prismDialect, m, false)
	b.WriteString("vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {\n  VertexOut out;\n")
	for _, s := range m.Vertex.Body {
		if vs.Varyings[s.Target] {
			fmt.Fprintf(&b, "  out.%s = %s;\n", s.Target, ir.Print(s.Value, vs))
		} else {
			fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, vs))
		}
	}
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	fs := internal.NewQualified(prismDialect, m, true)
	fragSig := "fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]"
	for i, t := range m.Textures {
		if t.Cube {
			fragSig += fmt.Sprintf(", texturecube<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
		} else {
			fragSig += fmt.Sprintf(", texture2d<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
		}
	}
	b.WriteString(fragSig + ") {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// emitMeshAuthored emits a mesh material that authors its own vertex() stage (B4).
//
// The vertex function takes its index source from [[vertex_id]] (procedural
// geometry) and/or per-vertex [[stage_in]] attributes, computes the clip-space
// position from the author body, and writes author varyings into VertexOut. The
// fragment function reads them. An optional statefield adds a read-only inState
// buffer + a StateGrid uniform addressed by stateAt(uv). Legacy mesh materials go
// through emitMesh and stay byte-identical.
func emitMeshAuthored(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	// Mesh always has the mvp/normalMatrix transform uniforms, so Uniforms exists.
	b.WriteString("struct Uniforms {\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "  %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("};\n\n")

	if m.StateField != "" {
		b.WriteString("struct StateGrid {\n  uint gridWidth;\n  uint gridHeight;\n};\n\n")
	}

	if len(m.Attributes) > 0 {
		b.WriteString("struct VertexIn {\n")
		for i, a := range m.Attributes {
			fmt.Fprintf(&b, "  %s %s [[attribute(%d)]];\n", typeName(a.Type), a.Name, i)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString("struct VertexOut {\n  float4 position [[position]];\n")
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "  %s %s;\n", typeName(v.Type), v.Name)
	}
	b.WriteString("};\n\n")

	// Vertex stage.
	vs := internal.NewQualified(prismDialect, m, false)
	vs.StateSampleUVFn = metalStateSampleUV
	var vparams []string
	if m.UsesVertexIndex {
		vparams = append(vparams, "uint vertexIndex [[vertex_id]]")
	}
	if len(m.Attributes) > 0 {
		vparams = append(vparams, "VertexIn in [[stage_in]]")
	}
	vparams = append(vparams, "constant Uniforms& u [[buffer(0)]]")
	if m.StateField != "" {
		vparams = append(vparams, "constant StateGrid& _stateGrid [[buffer(1)]]", "const device float4* _inState [[buffer(2)]]")
	}
	fmt.Fprintf(&b, "vertex VertexOut vertexMain(%s) {\n  VertexOut out;\n", strings.Join(vparams, ", "))
	internal.EmitStmtList(&b, m.Vertex.Body, vs, "  ", false)
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	// Fragment stage.
	fs := internal.NewQualified(prismDialect, m, true)
	fs.StateSampleUVFn = metalStateSampleUV
	fragSig := "fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]"
	for i, t := range m.Textures {
		if t.Cube {
			fragSig += fmt.Sprintf(", texturecube<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
		} else {
			fragSig += fmt.Sprintf(", texture2d<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
		}
	}
	if m.StateField != "" {
		fragSig += ", constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]"
	}
	b.WriteString(fragSig + ") {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// metalStateSampleUV renders a stateAt(uv) read against the inState buffer,
// addressing the grid by a uv -> linear cell index using the StateGrid dims. B4.
func metalStateSampleUV(uv string) string {
	return fmt.Sprintf(
		"_inState[min(uint((%s).x * float(_stateGrid.gridWidth)) + uint((%s).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)]",
		uv, uv)
}

// emitPoints emits a Metal points/particle surface.
// User uniforms are at [[buffer(1)]]; engine structs are at [[buffer(2)]].
func emitPoints(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	// Engine structs.
	b.WriteString(`struct FrameUniforms {
  float4x4 viewMatrix;
  float4x4 projMatrix;
  float viewportWidth;
  float viewportHeight;
};

struct PointsUniforms {
  float4x4 modelMatrix;
  float4 defaultColorAndSize;
  uint4 flags;
  float4 params;
  float4 fogColor;
};

struct ParticleInstance {
  float3 position;
  float size;
  float4 color;
};

`)

	if len(m.Uniforms) > 0 {
		b.WriteString("struct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString(`struct PointsOut {
  float4 clipPos [[position]];
  float3 v_color;
  float  v_fogFactor;
  float  v_alpha;
  float2 v_pointCoord;
  float  v_pointSize;
};

constant float2 _quadPos[6] = {
  {-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5},
  { 0.5, -0.5}, {0.5,  0.5}, {-0.5, 0.5},
};

vertex PointsOut vertexMain(
  uint vertexIndex   [[vertex_id]],
  uint instanceIndex [[instance_id]],
  constant FrameUniforms&  frame     [[buffer(0)]],
  constant PointsUniforms& pts       [[buffer(2)]],
  const device ParticleInstance* particles [[buffer(3)]]
`)
	if len(m.Uniforms) > 0 {
		b.WriteString(", constant UserUniforms& u [[buffer(1)]]")
	}
	b.WriteString(`) {
  float2 quad = _quadPos[vertexIndex];
  ParticleInstance p = particles[instanceIndex];
  float3 worldPos = (pts.modelMatrix * float4(p.position, 1.0)).xyz;
  float4 viewPos  = frame.viewMatrix * float4(worldPos, 1.0);
  float rawSize = (pts.flags.y == 0u) ? pts.defaultColorAndSize.w : p.size;
  float pixelSize;
  if (pts.flags.z != 0u) {
    pixelSize = max(rawSize * (frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  float minPx = max(pts.fogColor.w, 0.0);
  if (minPx > 0.0) pixelSize = max(pixelSize, minPx);
  if (pts.params.w > 0.0) pixelSize = min(pixelSize, pts.params.w);
  float4 clipPos = frame.projMatrix * viewPos;
  float2 viewport = max(float2(frame.viewportWidth, frame.viewportHeight), float2(1.0));
  float ndcX = quad.x * pixelSize / viewport.x * clipPos.w * 2.0;
  float ndcY = quad.y * pixelSize / viewport.y * clipPos.w * 2.0;
  PointsOut out;
  out.clipPos = float4(clipPos.x + ndcX, clipPos.y + ndcY, clipPos.z, clipPos.w);
  out.v_color = (pts.flags.x != 0u) ? p.color.rgb : pts.defaultColorAndSize.rgb;
  out.v_alpha = p.color.a * pts.params.x;
  out.v_pointCoord = quad + float2(0.5, 0.5);
  out.v_pointSize = pixelSize;
  if (pts.params.y != 0.0) {
    float dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-pts.params.z * pts.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

`)

	// Fragment stage.
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
	fragSig := "fragment float4 fragmentMain(PointsOut in [[stage_in]]"
	if len(m.Uniforms) > 0 {
		fragSig += ", constant UserUniforms& u [[buffer(1)]]"
	}
	b.WriteString(fragSig + ") {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}

// emitPost emits a Metal post-process pass surface.
// Engine textures: sceneColor at [[texture(0)]], sceneDepth at [[texture(2)]].
// User uniforms at [[buffer(1)]].
func emitPost(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString(`struct PostOut {
  float4 pos [[position]];
  float2 v_uv;
};

vertex PostOut vertexMain(uint vid [[vertex_id]]) {
  const float2 pos[3] = { {-1,-1},{3,-1},{-1,3} };
  const float2 uvs[3] = { {0,1},{2,1},{0,-1} };
  PostOut out;
  out.pos  = float4(pos[vid], 0, 1);
  out.v_uv = uvs[vid];
  return out;
}

`)

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
				return fmt.Sprintf("_sceneColorTex.sample(_sceneColorSamp, %s)", uv)
			case "sceneDepth":
				return fmt.Sprintf("_sceneDepthTex.sample(_sceneDepthSamp, %s)", uv)
			default:
				return "float4(0)"
			}
		},
		SceneSampleLevelFn: func(name, uv, lod string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("_sceneColorTex.sample(_sceneColorSamp, %s, level(%s))", uv, lod)
			case "sceneDepth":
				return fmt.Sprintf("_sceneDepthTex.sample(_sceneDepthSamp, %s, level(%s))", uv, lod)
			default:
				return "float4(0)"
			}
		},
		SceneSizeFn: func() string {
			return "float2(_sceneColorTex.get_width(), _sceneColorTex.get_height())"
		},
	}

	fragSig := "fragment float4 fragmentMain(PostOut in [[stage_in]], texture2d<float> _sceneColorTex [[texture(0)]], sampler _sceneColorSamp [[sampler(0)]], depth2d<float> _sceneDepthTex [[texture(2)]], sampler _sceneDepthSamp [[sampler(2)]]"
	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		fragSig += ", constant UserUniforms& u [[buffer(1)]]"
	}
	b.WriteString(fragSig + ") {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}

// typeName spells an ir.Type in Metal, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
