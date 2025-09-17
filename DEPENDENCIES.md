# Dependency Documentation

## Direct Dependencies

### `github.com/opentofu/tofudl v0.0.1`
- **Purpose**: OpenTofu Downloader Library (only approved third-party dependency)
- **Usage**: Downloads, verifies, and manages OpenTofu binary installations
- **Used in**: `pkg/opentofu/client.go`

## Indirect Dependencies

All indirect dependencies come from `github.com/opentofu/tofudl` for secure OpenTofu binary management:

### Cryptographic Verification (ProtonMail ecosystem)
- `github.com/ProtonMail/go-crypto` - OpenPGP implementation
- `github.com/ProtonMail/go-mime` - MIME handling for signed content
- `github.com/ProtonMail/gopenpgp/v2` - High-level OpenPGP wrapper

### Additional Security
- `github.com/cloudflare/circl` - Cloudflare's cryptographic library
- `golang.org/x/crypto` - Go's extended cryptography package

### System & Utilities
- `github.com/pkg/errors` - Enhanced error handling
- `golang.org/x/sys` - Platform-specific system calls
- `golang.org/x/text` - Text processing

## Why So Many Crypto Dependencies?

OpenTofu releases are cryptographically signed for security. The `tofudl` library:
1. Downloads OpenTofu binaries from official releases
2. Verifies digital signatures to ensure authenticity
3. Prevents installation of tampered or malicious binaries

This follows security best practices for external binary management.