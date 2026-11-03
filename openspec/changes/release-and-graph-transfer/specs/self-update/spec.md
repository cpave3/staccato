## ADDED Requirements

### Requirement: Finding the Newest Release

`st update` SHALL ask GitHub for the latest release of `cpave3/staccato`, which is never a draft or a prerelease, without a token. It SHALL be the only command that contacts GitHub's releases, and no other command SHALL check for updates.

#### Scenario: Newer release exists
- **WHEN** the running binary is `v0.4.2` and the latest release is `v0.5.0`
- **THEN** `st update` MUST install `v0.5.0`

#### Scenario: Already the newest
- **WHEN** the running binary is the latest release, or newer
- **THEN** `st update` MUST print that it is up to date, change nothing, and exit 0

#### Scenario: GitHub unreachable or refusing
- **WHEN** the request for the latest release fails or GitHub answers with an error
- **THEN** `st update` MUST print the reason, change nothing, and exit non-zero

#### Scenario: Other commands stay offline
- **WHEN** any command other than `st update` runs
- **THEN** it MUST NOT contact GitHub's releases

### Requirement: Installing a Release

`st update` SHALL install the release's asset for the running OS and CPU (`st-<os>-<arch>`) over the running binary, following links to the file they lead to. It SHALL download the asset into the same directory as that file, check its SHA-256 against the release's `SHA256SUMS`, make it executable, and run it with `version --json`, which MUST report the release's version. Only then SHALL it rename the running binary to `<file>.prev`, replacing an older one, and rename the new file into its place. Any failure SHALL leave the running binary as it was and remove the downloaded file.

#### Scenario: Successful install through a link
- **WHEN** `~/.local/bin/st` is a link to `/opt/st/st` and a newer release exists
- **THEN** the new binary MUST be written to `/opt/st/st`
- **AND** the previous one MUST be kept as `/opt/st/st.prev`
- **AND** the link MUST be left as it was

#### Scenario: Checksum mismatch
- **WHEN** the downloaded asset's SHA-256 differs from its line in `SHA256SUMS`
- **THEN** `st update` MUST fail naming the mismatch and leave the running binary unchanged

#### Scenario: Asset missing for this platform
- **WHEN** the release has no asset named for the running OS and CPU
- **THEN** `st update` MUST fail naming the asset it looked for and leave the running binary unchanged

#### Scenario: New binary reports another version
- **WHEN** the downloaded binary's `version --json` does not report the release's version
- **THEN** `st update` MUST fail and leave the running binary unchanged

#### Scenario: Directory not writable
- **WHEN** the directory of the binary cannot be written
- **THEN** `st update` MUST fail naming the directory and leave the running binary unchanged

### Requirement: Builds That Do Not Update

A binary whose `st version --json` reports `release: false` SHALL refuse to update itself, exit non-zero, and say how it was built and how to update it instead (`go install github.com/cpave3/staccato/cmd/st@latest` for a module version, rebuilding from the checkout for `dev`).

#### Scenario: Built from a checkout
- **WHEN** `st update` runs in a `dev` build
- **THEN** it MUST refuse without contacting GitHub and leave the binary unchanged

### Requirement: Check Only

`st update --check` SHALL report the running version, the latest release, and whether it is newer, without downloading or changing anything. With `--json` it SHALL print `{"current", "latest", "available", "url"}`. It SHALL work in any build, `dev` included.

#### Scenario: Newer release available
- **WHEN** `st update --check --json` runs in `v0.4.2` and the latest release is `v0.5.0`
- **THEN** it MUST print `available: true` with the release's address and exit 0
