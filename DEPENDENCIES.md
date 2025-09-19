# Dependency Documentation

## Direct Dependencies

### `github.com/opentofu/tofudl v0.0.1`
- **Purpose**: OpenTofu Downloader Library (only approved third-party dependency)
- **Usage**: Downloads, verifies, and manages OpenTofu binary installations
- **Used in**: `pkg/opentofu/client.go`

## Indirect Dependencies

All indirect dependencies come from `github.com/opentofu/tofudl` for secure OpenTofu binary management:

### Cryptographic Verification (ProtonMail ecosystem)
- `github.com/ProtonMail/go-crypto v1.3.0` - OpenPGP implementation
- `github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f` - MIME handling for signed content
- `github.com/ProtonMail/gopenpgp/v2 v2.9.0` - High-level OpenPGP wrapper

### Additional Security
- `github.com/cloudflare/circl v1.6.1` - Cloudflare's cryptographic library
- `golang.org/x/crypto v0.42.0` - Go's extended cryptography package

### System & Utilities
- `github.com/pkg/errors v0.9.1` - Enhanced error handling
- `golang.org/x/sys v0.36.0` - Platform-specific system calls
- `golang.org/x/text v0.29.0` - Text processing

## Why So Many Crypto Dependencies?

OpenTofu releases are cryptographically signed for security. The `tofudl` library:
1. Downloads OpenTofu binaries from official releases
2. Verifies digital signatures to ensure authenticity
3. Prevents installation of tampered or malicious binaries

This follows security best practices for external binary management.