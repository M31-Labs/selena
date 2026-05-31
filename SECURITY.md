# Security Policy

Selena compiles shader material source into backend shader strings and host
binding descriptors. Treat untrusted `.sel` input the same way you would treat
untrusted source code: parse and compile it in a bounded process, and do not
execute generated shaders outside the renderer sandbox you already trust.

## Reporting

Please report security issues privately through the GitHub security advisory
flow for this repository. If that is unavailable, contact M31 Labs through the
owner profile linked from the repository.

Do not open a public issue for a vulnerability until there is a coordinated fix
or mitigation.
