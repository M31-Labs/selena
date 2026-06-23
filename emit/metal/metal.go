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
// fragmentMain, for the iOS SceneKit backend consumed by gosx-native.
func Emit(m ir.Module) (string, error) {
	switch m.Kind {
	case ir.KindPoints:
		return emitPoints(m)
	case ir.KindPost:
		return emitPost(m)
	default:
		return emitMesh(m)
	}
}

// emitMesh is the original mesh pipeline emitter.
// NOTE: this emits standalone MSL functions. The SceneKit binding strategy
// (SCNProgram semantic uniforms vs. a packed argument buffer) is the binding-
// model design decision the slice is meant to expose; for now uniforms are a
// single constant buffer at [[buffer(0)]].
func emitMesh(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	if len(m.Uniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
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
		fragSig += fmt.Sprintf(", texture2d<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
	}
	b.WriteString(fragSig + ") {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
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
  float ndcX = quad.x * pixelSize / frame.viewportWidth  * clipPos.w * 2.0;
  float ndcY = quad.y * pixelSize / frame.viewportHeight * clipPos.w * 2.0;
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
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}

// emitPost emits a Metal post-process pass surface.
// Engine textures: sceneColor at [[texture(0)]], sceneDepth at [[texture(2)]].
// User uniforms at [[buffer(1)]].
func emitPost(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	if len(m.Uniforms) > 0 {
		b.WriteString("struct UserUniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
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

	varyingSet := map[string]bool{"v_uv": true}
	fs := internal.Resolver{
		Dialect:   prismDialect,
		Uniforms:  internal.NameSet(m.Uniforms),
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
	}

	fragSig := "fragment float4 fragmentMain(PostOut in [[stage_in]], texture2d<float> _sceneColorTex [[texture(0)]], sampler _sceneColorSamp [[sampler(0)]], depth2d<float> _sceneDepthTex [[texture(2)]], sampler _sceneDepthSamp [[sampler(2)]]"
	if len(m.Uniforms) > 0 {
		fragSig += ", constant UserUniforms& u [[buffer(1)]]"
	}
	b.WriteString(fragSig + ") {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}

// typeName spells an ir.Type in Metal, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
