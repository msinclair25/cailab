# Third-party notices

CloudAILab release binaries include the Go runtime and packages from the modules listed in [`third_party/modules.txt`](third_party/modules.txt). The inventory is derived from the packages linked into `./cmd/cailab`, not every module downloaded for tests or tooling.

The corresponding license texts, copyright statements, and upstream notices are preserved under [`third_party/licenses`](third_party/licenses). Apache-2.0 dependencies may also rely on the complete Apache-2.0 text in the repository-level [`LICENSE`](LICENSE). Release archives include all three locations.

| Component | Version | License evidence |
|---|---:|---|
| Go runtime and standard library | 1.25.12 | BSD-3-Clause; `third_party/licenses/go/LICENSE` |
| AWS SDK for Go v2 modules | See `third_party/modules.txt` | Apache-2.0 plus copied BSD-3-Clause `singleflight`; `third_party/licenses/aws-sdk-go-v2/` |
| AWS Smithy for Go | 1.27.3 | Apache-2.0 plus copied BSD-3-Clause `singleflight`; `third_party/licenses/aws-smithy-go/` |
| `github.com/dustin/go-humanize` | 1.0.1 | MIT; `third_party/licenses/dustin-go-humanize/LICENSE` |
| `github.com/google/uuid` | 1.6.0 | BSD-3-Clause; `third_party/licenses/google-uuid/LICENSE` |
| `github.com/mattn/go-isatty` | 0.0.20 | MIT; `third_party/licenses/mattn-go-isatty/LICENSE` |
| `github.com/ncruces/go-strftime` | 1.0.0 | MIT; `third_party/licenses/ncruces-go-strftime/LICENSE` |
| `github.com/remyoudompheng/bigfft` | pseudo-version ending `24d4a6f8daec` | BSD-3-Clause; `third_party/licenses/remyoudompheng-bigfft/LICENSE` |
| `github.com/santhosh-tekuri/jsonschema/v6` | 6.0.2 | Apache-2.0; `third_party/licenses/santhosh-jsonschema-v6/LICENSE` |
| `go.yaml.in/yaml/v3` | 3.0.4 | MIT with preserved upstream Apache-2.0 notice; `third_party/licenses/go-yaml-v3/` |
| `golang.org/x/sys` | 0.44.0 | BSD-3-Clause; `third_party/licenses/golang-x-sys/LICENSE` |
| `golang.org/x/text` | 0.14.0 | BSD-3-Clause; `third_party/licenses/golang-x-text/LICENSE` |
| `modernc.org/libc` | 1.73.4 | BSD-3-Clause plus bundled permissive third-party notices; `third_party/licenses/modernc-libc/` |
| `modernc.org/mathutil` | 1.7.1 | BSD-3-Clause; `third_party/licenses/modernc-mathutil/LICENSE` |
| `modernc.org/memory` | 1.11.0 | BSD-3-Clause and copied Go/mmap notices; `third_party/licenses/modernc-memory/` |
| `modernc.org/sqlite` | 1.53.0 | BSD-3-Clause wrapper and public-domain SQLite notice; `third_party/licenses/modernc-sqlite/` |

Floci and Docker are separately obtained runtimes; their files are not bundled into CloudAILab release archives. Their reviewed versions, digests, licenses, and compatibility limits are recorded in the [technical basis](docs/06-research/technical-basis.md) and provider compatibility documentation.

This notice inventory is an engineering record, not legal advice. Dependency changes must update the linked-module inventory and applicable license material in the same change.
