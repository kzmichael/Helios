# TLS & PKI Foundations

> **Core principle:** A certificate binds an identity to a public key. A Certificate Authority (CA) signs that binding. A certificate chain connects the certificate to a trusted root, and the client's trust store determines whether that root is trusted.

---

# 1. What Problem Does TLS Solve?

When connecting to:

```text
https://api.example.com
```

the client needs to establish:

1. **Identity** — Am I actually talking to `api.example.com`?
2. **Confidentiality** — Can someone read the traffic?
3. **Integrity** — Can someone modify the traffic without detection?
4. **Trust** — Why should I trust the server's certificate?

TLS uses:

```text
Asymmetric cryptography
        +
Certificates
        +
Certificate Authorities
        +
Certificate chains
        +
Trust stores
        +
Symmetric session encryption
```

---

# 2. Symmetric Cryptography

Symmetric encryption uses the **same secret key** for encryption and decryption.

```text
              SAME SECRET KEY
                     │
                     ▼
Message ──encrypt──> Ciphertext
                         │
                         │ decrypt
                         ▼
                      Message
```

The major problem is:

> How do two parties securely obtain the same secret key?

Asymmetric cryptography helps solve the initial authentication and key-establishment problem.

---

# 3. Asymmetric Cryptography

Asymmetric cryptography uses a key pair:

```text
Public Key
+
Private Key
```

The keys are mathematically related.

```text
             KEY PAIR
        ┌───────────────┐
        │               │
   Public Key      Private Key
        │               │
      share           protect
```

### Public key

Can be shared publicly.

### Private key

Must remain secret.

---

# 4. Public Key vs Private Key

One simplified use of asymmetric cryptography:

```text
Sender
   │
   │ encrypt using recipient's PUBLIC KEY
   ▼
Ciphertext
   │
   │ decrypt using recipient's PRIVATE KEY
   ▼
Recipient
```

Another important use is digital signatures:

```text
Private Key
     │
     ▼
   Sign
     │
     ▼
Signature
     │
     ▼
Public Key
     │
     ▼
Verify
```

Therefore remember:

```text
Public key  → can be shared
Private key → must be protected
```

---

# 5. Why the Private Key Matters

Suppose:

```text
api.example.com
```

has:

```text
server.crt
server.key
```

The certificate can be publicly distributed.

The private key must be protected.

```text
server.crt → public
server.key → SECRET
```

If an attacker obtains the private key, they may be able to impersonate the service, depending on the protocol and certificate status.

---

# 6. What Is a Certificate?

An X.509 certificate is a signed data structure that essentially says:

> "This public key is associated with this identity, according to the issuer's validation rules."

A simplified certificate contains:

```text
Certificate
├── Subject / identity
├── Public key
├── Issuer
├── Validity period
├── Extensions
└── CA signature
```

The certificate is **not the private key**.

---

# 7. Certificate Authority — CA

A **Certificate Authority (CA)** issues/signs certificates.

Conceptually:

```text
CA
 │
 │ signs
 ▼
Server Certificate
 │
 └── api.example.com
```

The CA is asserting that the certificate was issued according to its validation rules.

But this creates another question:

> Why does the client trust the CA?

The answer is the **trust store**.

---

# 8. Certificate Chains

Certificates commonly form a hierarchy:

```text
Root CA
   │
   ▼
Intermediate CA
   │
   ▼
Server Certificate
```

For example:

```text
Root CA
   ↓ signs
Intermediate CA
   ↓ signs
api.example.com
```

This is a **certificate chain**.

---

# 9. Root CA

A root CA is a **trust anchor**.

Operating systems and browsers maintain trusted root certificates.

Conceptually:

```text
Trust Store
├── Root CA A
├── Root CA B
├── Root CA C
└── Root CA D
```

If the client's validation process can build a valid chain to one of these trusted roots, the certificate may be trusted, assuming all other validation checks pass.

---

# 10. Intermediate CA

An intermediate CA sits between the root and leaf/server certificate.

```text
Trusted Root CA
       │
       │ signs
       ▼
Intermediate CA
       │
       │ signs
       ▼
Server Certificate
```

The intermediate CA allows certificate issuance to be delegated without using the root CA directly for every certificate.

---

# 11. Server / Leaf Certificate

The final certificate represents the server identity.

Example:

```text
api.example.com
```

Conceptually:

```text
Server Certificate
├── Identity
├── Public Key
├── Issuer
├── Valid From
├── Valid Until
├── Extensions
└── CA Signature
```

The server also possesses the corresponding private key.

---

# 12. Complete Certificate Chain

Suppose:

```text
api.example.com
       │
       ▼
Intermediate CA
       │
       ▼
Trusted Root CA
```

The server commonly sends:

```text
Leaf certificate
+
Intermediate certificate
```

The client already has the trusted root in its trust store.

Conceptually:

```text
SERVER SENDS
─────────────
Leaf certificate
Intermediate certificate

CLIENT ALREADY HAS
──────────────────
Trusted Root CA
```

The client builds:

```text
Leaf
 ↓
Intermediate
 ↓
Trusted Root
```

and validates the chain.

---

# 13. Trust Store

A trust store contains CA certificates that a client considers trusted.

Conceptually:

```text
Client Trust Store
├── Root CA 1
├── Root CA 2
├── Root CA 3
└── Root CA 4
```

When a server presents a certificate, the client asks:

> Can I build a valid chain from this certificate to a trusted root?

If yes, and all other checks pass, the certificate can be trusted.

---

# 14. OS Trust Store vs Application Trust Store

Not every application necessarily uses exactly the same trust configuration.

Possible model:

```text
Application
    │
    ├── OS trust store
    │
    └── Application-specific CA bundle
```

This is extremely important in DevOps.

You can have:

```text
Browser → certificate trusted
Container application → certificate NOT trusted
```

because they may use different trust stores.

---

# 15. Container Trust Store Problem

Suppose your laptop has:

```text
Corporate Root CA
```

installed.

Your browser can access:

```text
https://internal.company.com
```

But your container has:

```text
Application
    ↓
Container CA bundle
    ↓
Corporate Root CA missing
```

The application may fail with:

```text
CERTIFICATE_VERIFY_FAILED
```

The certificate itself may be perfectly valid.

The problem is:

> The container doesn't trust the issuing CA.

---

# 16. X.509

X.509 is a standard for public-key certificates.

Important fields include:

```text
Subject
Issuer
Validity
Public Key
Extensions
Signature
```

Two fields that frequently cause confusion:

```text
CN
SAN
```

---

# 17. CN — Common Name

Historically, the Common Name was commonly used to represent the certificate's hostname.

Example:

```text
CN = api.example.com
```

Older systems might use the CN during hostname verification.

However:

> Modern hostname verification relies on SAN.

---

# 18. SAN — Subject Alternative Name

SAN allows a certificate to specify identities.

Example:

```text
Subject Alternative Name:

DNS:api.example.com
DNS:www.example.com
DNS:example.com
```

One certificate can therefore cover multiple DNS names.

---

# 19. CN vs SAN

Remember:

```text
CN
│
└── Historical / legacy hostname field

SAN
│
└── Modern hostname identity mechanism
```

For modern TLS hostname verification:

> **SAN is what matters.**

Example:

```text
Certificate:

CN:
example.com

SAN:
api.example.com
```

Connecting to:

```text
https://api.example.com
```

uses the SAN identity for hostname verification.

---

# 20. Wildcard SAN

A certificate can contain:

```text
SAN:
DNS:*.example.com
```

This can generally cover:

```text
api.example.com
www.example.com
login.example.com
```

Wildcard matching has defined rules and limitations.

For example:

```text
*.example.com
```

does not simply mean:

```text
anything.anything.example.com
```

---

# 21. Certificate Signing Request — CSR

Suppose you want a certificate for:

```text
api.company.com
```

First generate:

```text
Private Key
```

Then create the corresponding:

```text
Public Key
```

Then generate a:

```text
CSR
```

The CSR is sent to the CA.

---

# 22. CSR Process

```text
Generate Private Key
        │
        ▼
Generate Public Key
        │
        ▼
Create CSR
        │
        ▼
Send CSR to CA
        │
        ▼
CA validates request
        │
        ▼
CA signs certificate
        │
        ▼
Certificate returned
```

The private key normally stays with the requester.

---

# 23. What Does a CSR Contain?

A CSR can contain:

```text
Subject
Public Key
Requested extensions
Signature
```

Example:

```text
Subject:
CN = api.company.com

SAN:
DNS:api.company.com
DNS:www.company.com

Public Key:
...

Signature:
...
```

The CSR is signed using the corresponding private key.

This allows the CA to verify that the requester possesses the private key associated with the public key in the request.

---

# 24. CSR ≠ Certificate

This distinction is extremely important.

### CSR

```text
"I am requesting a certificate for this identity/public key."
```

### Certificate

```text
"The CA has issued a signed certificate binding this identity to this public key."
```

Flow:

```text
Private Key
    │
    ├──────► Public Key
    │
    └──────► CSR
               │
               ▼
              CA
               │
               ▼
          Certificate
```

---

# 25. What Does the CA Sign?

Conceptually:

```text
Certificate Data
      │
      ▼
CA Private Key
      │
      ▼
Digital Signature
```

The resulting certificate contains:

```text
Certificate Data
+
CA Signature
```

The client verifies the signature using the CA's public key.

---

# 26. Full HTTPS Validation

Suppose you visit:

```text
https://api.example.com
```

The server presents its certificate chain.

The client performs several checks.

### Check 1 — Hostname

```text
Requested hostname:
api.example.com

Certificate SAN:
api.example.com
```

Match → good.

---

### Check 2 — Validity

Check:

```text
Not Before
Not After
```

The current time must fall within the validity period.

---

### Check 3 — Certificate chain

```text
Leaf
 ↓
Intermediate
 ↓
Trusted Root
```

The chain must validate.

---

### Check 4 — Trust

The root must ultimately be trusted by the client.

---

### Check 5 — Private key possession

During TLS, the server proves possession of the private key corresponding to the public key in the certificate.

---

### Check 6 — TLS session establishment

TLS establishes symmetric session keys.

---

### Check 7 — Encrypted communication

```text
Client
   ║
   ║ encrypted TLS session
   ║
Server
```

---

# 27. Why TLS Uses Symmetric Encryption for the Session

Asymmetric cryptography is relatively expensive.

TLS therefore follows the general pattern:

```text
Asymmetric cryptography
        ↓
Authentication / key establishment
        ↓
Symmetric session keys
        ↓
Fast encrypted communication
```

The actual HTTP traffic is encrypted using symmetric cryptography.

---

# 28. Certificate vs Private Key

Do not confuse these.

```text
Certificate
───────────
Public
Contains public key
Contains identity information
Signed by CA

Private Key
───────────
Secret
Used for cryptographic operations
Must be protected
```

Typical server files:

```text
tls.crt
tls.key
```

Conceptually:

```text
tls.crt → certificate / certificate chain
tls.key → private key
```

---

# 29. Kubernetes Connection

Eventually you'll see:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-tls
type: kubernetes.io/tls
data:
  tls.crt: ...
  tls.key: ...
```

Now you know what these mean:

```text
tls.crt
   ↓
Certificate + potentially certificate chain

tls.key
   ↓
Private key
```

An Ingress controller can use these to terminate TLS.

---

# 30. TLS Termination

Example architecture:

```text
Internet
    │
    │ HTTPS
    ▼
Load Balancer / Ingress
    │
    │ TLS termination
    ▼
HTTP / HTTPS
    │
    ▼
Service
    │
    ▼
Pod
```

The certificate/private key may therefore live at the TLS termination point rather than inside the application itself.

---

# 31. Certificate Chain in Kubernetes

Example:

```text
Root CA
   ↓
Intermediate CA
   ↓
api.company.com
```

The TLS certificate presented by the server commonly contains:

```text
api.company.com certificate
+
Intermediate CA certificate
```

The trusted root normally comes from the client's trust store.

---

# 32. Common TLS Errors

## Certificate expired

```text
certificate expired
```

Check:

```text
Not Before
Not After
```

---

## Hostname mismatch

Example:

```text
Requested:
api.company.com

SAN:
web.company.com
```

Result:

```text
certificate is not valid for this name
```

Check SAN.

---

## Unknown CA

The client doesn't trust the issuing CA.

Check:

```text
Trust store
Certificate chain
Corporate/internal CA
```

---

## Unable to get local issuer certificate

Often means the client cannot construct a valid chain.

Check:

```text
Leaf
 ↓
Intermediate
 ↓
Trusted Root
```

The server may be missing an intermediate certificate.

---

## CERTIFICATE_VERIFY_FAILED

Broad error.

Investigate:

```text
Hostname
Validity dates
Certificate chain
Trust store
Certificate usage
System time
```

---

# 33. TLS Troubleshooting Framework

When TLS fails, ask these questions in order:

```text
1. WHO?
   ↓
   Does SAN match the hostname?

2. WHEN?
   ↓
   Is the certificate currently valid?

3. WHO SIGNED IT?
   ↓
   Is the issuer trusted?

4. CAN I BUILD THE CHAIN?
   ↓
   Leaf → Intermediate → Root

5. WHAT DOES THE CLIENT TRUST?
   ↓
   Check trust store / CA bundle

6. DOES THE SERVER OWN THE PRIVATE KEY?
   ↓
   Verify key/certificate pairing

7. IS THE CLIENT USING THE EXPECTED TLS CONFIG?
   ↓
   Protocol / cipher / client authentication requirements
```

---

# 34. Complete Mental Model

```text
                  TLS / PKI
                     │
                     ▼
             Generate Private Key
                     │
                     ▼
                Public Key
                     │
                     ▼
                   CSR
                     │
                     ▼
                    CA
                     │
                  signs
                     ▼
             Server Certificate
                     │
          ┌──────────┴──────────┐
          │                     │
       Identity              Public Key
          │
         SAN
          │
          ▼
    Certificate Chain
          │
    ┌─────┴─────┐
    ▼           ▼
Intermediate   Root
                  │
                  ▼
           Client Trust Store
                  │
                  ▼
             Trust Decision
                  │
                  ▼
           TLS Authentication
                  │
                  ▼
           Session Key Setup
                  │
                  ▼
         Encrypted HTTP Traffic
```

---

# 35. Critical Distinctions

## Public Key vs Private Key

```text
Public Key
→ share

Private Key
→ protect
```

---

## CSR vs Certificate

```text
CSR
→ certificate request

Certificate
→ CA-signed identity/public-key binding
```

---

## CN vs SAN

```text
CN
→ historical/legacy hostname field

SAN
→ modern hostname verification
```

---

## Root vs Intermediate

```text
Root
→ trust anchor

Intermediate
→ delegated CA
```

---

## Certificate vs Trust

A certificate being mathematically valid does **not** automatically mean the client trusts it.

Successful validation generally requires:

```text
Valid certificate
       +
Correct hostname
       +
Valid dates
       +
Valid chain
       +
Trusted root
       +
Appropriate certificate usage
```

---

# 36. DevOps Mental Model

When working with TLS in infrastructure, think:

```text
DNS name
   ↓
Certificate SAN
   ↓
Certificate
   ↓
Private key
   ↓
Certificate chain
   ↓
CA
   ↓
Client trust store
   ↓
TLS handshake
   ↓
Encrypted connection
```

This model explains most of what you'll encounter later with:

```text
HTTPS
Ingress TLS
Kubernetes Secrets
cert-manager
Internal CAs
mTLS
Service Mesh
Certificate Rotation
API Gateways
Load Balancers
```

---

# 37. Revision Cheat Sheet

```text
┌───────────────────────────────────────────────────────────┐
│                  TLS / PKI CHEAT SHEET                    │
└───────────────────────────────────────────────────────────┘

ASYMMETRIC CRYPTO
──────────────────
Public Key  → share
Private Key → protect

Used for:
• Authentication
• Digital signatures
• Key establishment


SYMMETRIC CRYPTO
────────────────
Same secret key → encrypt + decrypt

Used for:
• Actual high-volume session traffic


CERTIFICATE
───────────
Binds:

Identity
   +
Public Key

and contains a CA signature.


CA
──
Certificate Authority

Issues/signs certificates.


CERTIFICATE CHAIN
─────────────────
Leaf certificate
      ↓
Intermediate CA
      ↓
Trusted Root CA


ROOT CA
───────
Trust anchor.

Usually already present in the client's trust store.


INTERMEDIATE CA
───────────────
Delegated CA.

Usually signs leaf/server certificates.


TRUST STORE
───────────
Collection of trusted CA certificates.

OS/browser/application may have different trust stores.


X.509
─────
Certificate standard.

Important fields:

Subject
Issuer
Validity
Public Key
Extensions
Signature


CN
──
Common Name.

Historical/legacy hostname field.


SAN
───
Subject Alternative Name.

Modern hostname verification.

Example:
DNS:api.example.com


CSR
───
Certificate Signing Request.

Contains:
• Subject
• Public key
• Requested extensions
• CSR signature

CSR = REQUEST
Certificate = ISSUED RESULT


TLS FLOW
────────
Client
  ↓
Server certificate
  ↓
Check SAN
  ↓
Check validity
  ↓
Build certificate chain
  ↓
Check trusted root
  ↓
Verify signatures
  ↓
Server proves private-key possession
  ↓
Establish session keys
  ↓
Encrypted communication


COMMON ERRORS
─────────────
Expired
→ validity dates

Hostname mismatch
→ SAN

Unknown CA
→ trust store

Unable to get local issuer
→ certificate chain

CERTIFICATE_VERIFY_FAILED
→ check hostname + dates + chain + trust


KUBERNETES
──────────
tls.crt → certificate / chain
tls.key → private key

Ingress / Load Balancer
→ may terminate TLS


GOLDEN RULE
───────────
Certificate = identity + public key + CA signature

Private key = secret

Trust store = what the client trusts

SAN = hostname identity

Chain = path from leaf → trusted root
```

---

# 38. One Sentence to Memorize

> **A certificate proves an identity is associated with a public key, a CA signs that association, the chain connects it to a trusted root, and the trust store determines whether the client accepts that root.**
