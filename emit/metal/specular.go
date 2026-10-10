package metal

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

func writeSpecularFragment(b *strings.Builder, m ir.Module, fs internal.Resolver) {
	params := []string{"VertexOut in", "constant Uniforms& u"}
	entry := []string{"VertexOut in [[stage_in]]", "constant Uniforms& u [[buffer(0)]]"}
	args := []string{"in", "u"}
	for i, t := range m.Textures {
		typ := "texture2d<float>"
		if t.Cube {
			typ = "texturecube<float>"
		}
		params = append(params, typ+" "+t.Name, "sampler "+t.Name+"Sampler")
		entry = append(entry, fmt.Sprintf("%s %s [[texture(%d)]]", typ, t.Name, i), fmt.Sprintf("sampler %sSampler [[sampler(%d)]]", t.Name, i))
		args = append(args, t.Name, t.Name+"Sampler")
	}
	if m.StateField != "" {
		params = append(params, "constant StateGrid& _stateGrid", "const device float4* _inState")
		entry = append(entry, "constant StateGrid& _stateGrid [[buffer(1)]]", "const device float4* _inState [[buffer(2)]]")
		args = append(args, "_stateGrid", "_inState")
	}
	if body, color, specular, ok := ir.FusedSpecular(m); ok {
		b.WriteString("struct SelenaFragmentOutput {\n  float4 color [[color(0)]];\n  float4 specular [[color(1)]];\n};\n\n")
		fmt.Fprintf(b, "fragment SelenaFragmentOutput fragmentMainSpecular(%s) {\n", strings.Join(entry, ", "))
		internal.EmitStmtList(b, body, fs, "  ", false)
		fmt.Fprintf(b, "  float4 selenaMRTColor = %s;\n  float3 selenaMRTSpecular = (%s).rgb;\n  return SelenaFragmentOutput{selenaMRTColor, float4(selenaMRTSpecular, selenaMRTColor.a)};\n}\n", ir.Print(color, fs), ir.Print(specular, fs))
		return
	}
	sig := strings.Join(params, ", ")
	internal.EmitSpecularHelpers(b, m, fs, "float4 selenaSurface("+sig+")", "float4 selenaSpecular("+sig+")", false)
	b.WriteString("struct SelenaFragmentOutput {\n  float4 color [[color(0)]];\n  float4 specular [[color(1)]];\n};\n\n")
	fmt.Fprintf(b, "fragment SelenaFragmentOutput fragmentMainSpecular(%s) {\n  float4 color = selenaSurface(%s);\n", strings.Join(entry, ", "), strings.Join(args, ", "))
	if m.Specular == nil {
		b.WriteString("  float3 specular = float3(0.0);\n")
	} else {
		fmt.Fprintf(b, "  float3 specular = selenaSpecular(%s).rgb;\n", strings.Join(args, ", "))
	}
	b.WriteString("  return SelenaFragmentOutput{color, float4(specular, color.a)};\n}\n")
}
