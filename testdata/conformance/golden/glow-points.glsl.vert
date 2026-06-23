attribute vec3 a_position;
attribute float a_size;
attribute vec4 a_color;

uniform mat4 u_viewMatrix;
uniform mat4 u_projectionMatrix;
uniform mat4 u_modelMatrix;
uniform float u_defaultSize;
uniform vec4 u_defaultColor;
uniform bool u_hasPerVertexColor;
uniform bool u_hasPerVertexSize;
uniform bool u_sizeAttenuation;
uniform float u_viewportHeight;
uniform float u_minPixelSize;
uniform float u_maxPixelSize;
uniform int u_hasFog;
uniform float u_fogDensity;
uniform float u_opacity;
uniform vec3 fogColor;

varying vec3 v_color;
varying float v_alpha;
varying float v_fogFactor;
varying float v_pointSize;

void main() {
  vec4 worldPos = u_modelMatrix * vec4(a_position, 1.0);
  vec4 viewPos = u_viewMatrix * worldPos;
  gl_Position = u_projectionMatrix * viewPos;
  float size = u_hasPerVertexSize ? a_size : u_defaultSize;
  float pixelSize;
  if (u_sizeAttenuation) {
    pixelSize = max(size * (u_viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(size, 1.0);
  }
  if (u_minPixelSize > 0.0) pixelSize = max(pixelSize, u_minPixelSize);
  if (u_maxPixelSize > 0.0) pixelSize = min(pixelSize, u_maxPixelSize);
  gl_PointSize = pixelSize;
  v_pointSize = pixelSize;
  vec4 _col = u_hasPerVertexColor ? a_color : u_defaultColor;
  v_color = _col.rgb;
  v_alpha = _col.a * u_opacity;
  if (u_hasFog != 0) {
    float dist = length(viewPos.xyz);
    v_fogFactor = clamp(exp(-u_fogDensity * u_fogDensity * dist * dist), 0.0, 1.0);
  } else {
    v_fogFactor = 1.0;
  }
}
