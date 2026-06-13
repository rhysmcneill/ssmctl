# Verify Build Provenance

All `ssmctl` releases are signed and come with SLSA provenance attestations. You can verify that a binary was built by our trusted CI/CD pipeline and hasn't been tampered with.

## Install the verifier

### Option 1: Install via Go

```bash
go install github.com/slsa-framework/slsa-verifier/v2/cli/slsa-verifier@v2.7.1
```

### Option 2: Install via Homebrew

```bash
brew install slsa-verifier
```

## Verify the .intoto.jsonl file

```bash
slsa-verifier verify-artifact ssmctl-linux-amd64 \
  --provenance-path ssmctl-linux-amd64.intoto.jsonl \
  --source-uri github.com/rhysmcneill/ssmctl \
  --source-tag vX.Y.Z
```

Replace vX.Y.Z with the version you have downloaded.

For additional and more in-depth explanation check out slsa-verifier's [documentation](https://github.com/slsa-framework/slsa-verifier/blob/main/README.md#verification-for-github-builders)
