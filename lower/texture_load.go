package lower

import (
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Data textures (spec G1): the load(tex, ix, iy) and size(tex) builtins, and the
// `format` qualifier on texture2d params. The rules live here so the shared
// typer, resolver and Lower only carry small hooks.

// textureArg checks that arg names a 2D texture param for the builtin fn and
// returns it. Cube and depth textures get their own messages, because the next
// thing the author needs is the call that does read them.
func textureArg(fn string, arg hir.Expr, paramKind map[string]hir.Type, span hir.Span) (hir.Ref, error) {
	ref, ok := arg.(hir.Ref)
	if !ok {
		return hir.Ref{}, diagnostic(CodeInvalidCall, span, "%s: first argument must be a texture2d param", fn)
	}
	switch paramKind[ref.Name] {
	case hir.Texture2D:
		return ref, nil
	case hir.TextureCube:
		return hir.Ref{}, diagnostic(CodeInvalidCall, span,
			"%s: %s is a textureCube; cube textures have no texel %s, read it with sampleCube(texture, dir) or sampleCubeLevel(texture, dir, lod)", fn, ref.Name, fn)
	case hir.TextureDepth2D, hir.TextureDepth2DArray:
		return hir.Ref{}, diagnostic(CodeInvalidCall, span,
			"%s: %s is a depth texture; read it with sampleDepth(texture, uv)", fn, ref.Name)
	}
	if ref.Name == "sceneColor" || ref.Name == "sceneDepth" {
		return hir.Ref{}, diagnostic(CodeInvalidCall, span,
			"%s: %s is an engine backdrop, not a texture2d param; use sceneColor(uv)", fn, ref.Name)
	}
	return hir.Ref{}, diagnostic(CodeInvalidCall, span, "%s: first argument must be a texture2d param", fn)
}

// typeTexelCall types load(tex, ix, iy) -> vec4 and size(tex) -> vec2.
func (t *typer) typeTexelCall(c hir.Call, kind builtinKind) (ir.Type, error) {
	if kind == builtinTextureSize {
		if len(c.Args) != 1 {
			return "", diagnostic(CodeInvalidCall, c.Span, "size(texture) takes 1 argument")
		}
		if _, err := textureArg("size", c.Args[0], t.paramKind, c.Span); err != nil {
			return "", err
		}
		return ir.Vec2, nil
	}
	if len(c.Args) != 3 {
		return "", diagnostic(CodeInvalidCall, c.Span, "load(texture, ix, iy) takes 3 arguments")
	}
	if _, err := textureArg("load", c.Args[0], t.paramKind, c.Span); err != nil {
		return "", err
	}
	for i, axis := range []string{"ix", "iy"} {
		at, err := t.typeOf(c.Args[i+1])
		if err != nil {
			return "", err
		}
		if at != ir.Int {
			return "", diagnostic(CodeTypeMismatch, c.Span,
				"load: %s must be an int texel coordinate, got %s (convert with int(x), or write an int literal such as 3i)", axis, at)
		}
	}
	return ir.Vec4, nil
}

// resolveTexelCall lowers load(tex, ix, iy) and size(tex) to ir.Load and
// ir.TextureSize. The typer has already checked the argument types on the
// normal path; the texture check is repeated because the resolver is also
// reached directly from a few lowering sites.
func (r *resolver) resolveTexelCall(x hir.Call) (ir.Expr, error) {
	if x.Func == "size" {
		if len(x.Args) != 1 {
			return nil, diagnostic(CodeInvalidCall, x.Span, "size(texture) takes 1 argument")
		}
		tex, err := textureArg("size", x.Args[0], r.paramKind, x.Span)
		if err != nil {
			return nil, err
		}
		return ir.TextureSize{Texture: tex.Name}, nil
	}
	if len(x.Args) != 3 {
		return nil, diagnostic(CodeInvalidCall, x.Span, "load(texture, ix, iy) takes 3 arguments")
	}
	tex, err := textureArg("load", x.Args[0], r.paramKind, x.Span)
	if err != nil {
		return nil, err
	}
	ix, err := r.expr(x.Args[1])
	if err != nil {
		return nil, err
	}
	iy, err := r.expr(x.Args[2])
	if err != nil {
		return nil, err
	}
	return ir.Load{Texture: tex.Name, X: ix, Y: iy}, nil
}

// checkTextureParams validates `format` qualifiers: only a texture2d param may
// declare one, and the name must be a known storage format.
func checkTextureParams(m hir.Material) error {
	for _, p := range m.Params {
		if p.Format == "" {
			continue
		}
		if p.Type != hir.Texture2D || p.IsArray {
			return diagnostic(CodeUnsupportedType, p.Span,
				"param %q: a format applies only to texture2d params, not %s", p.Name, p.Type)
		}
		if !ir.IsTextureFormat(p.Format) {
			return diagnostic(CodeUnsupportedType, p.Span,
				"param %q: unknown texture format %q (expected %s)", p.Name, p.Format, strings.Join(ir.TextureFormats, ", "))
		}
	}
	return nil
}

// stampTextureData copies each texture param's declared format onto the module
// and descriptor, sets the descriptor's sampleMode from how the material reads
// the texture, and rejects sampling a 32-bit float texture.
//
// A float32 texture cannot be filtered on WebGPU without the float32-filterable
// feature, so sample() and sampleLevel() on one would fail at bind-group
// creation on most devices. The ir.Module carries no source spans, so the
// diagnostic points at the param declaration.
func stampTextureData(m hir.Material, mod *ir.Module, layout *bindings.Layout) error {
	formats := map[string]hir.Param{}
	for _, p := range m.Params {
		if p.Format != "" {
			formats[p.Name] = p
		}
	}
	sampled, loaded := ir.TextureAccess(*mod)
	for _, p := range m.Params {
		if p.Format != "" && ir.IsFloat32Format(p.Format) && sampled[p.Name] {
			return diagnostic(CodeInvalidCall, p.Span,
				"texture %q has format %s, which WebGPU cannot filter without the float32-filterable feature; read it with load(%s, ix, iy) instead of sample() or sampleLevel()",
				p.Name, p.Format, p.Name)
		}
	}
	for i := range mod.Textures {
		name := mod.Textures[i].Name
		if p, ok := formats[name]; ok {
			mod.Textures[i].Format = p.Format
		}
	}
	for i := range layout.Textures {
		name := layout.Textures[i].Name
		if p, ok := formats[name]; ok {
			layout.Textures[i].Format = p.Format
		}
		switch {
		case loaded[name] && sampled[name]:
			layout.Textures[i].SampleMode = ir.SampleModeBoth
		case loaded[name]:
			layout.Textures[i].SampleMode = ir.SampleModeLoad
		}
	}
	return nil
}
