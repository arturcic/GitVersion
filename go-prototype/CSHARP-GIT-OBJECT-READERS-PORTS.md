# Git object reader contracts

Source checkpoint `b0145f065` ports 23 original C# methods and cases from the
loose object, packed object, annotated tag, and tree suites. The tests retain
the original identities, commit signatures and offset, messages, blob
contents, missing and wrong object types, alternate storage, nested tags,
and ordered tree metadata. Go uses go-git's tree entry lookup for the source
streaming lookup's found and missing outcomes.

The fixture verifies pack-only reads, a 26-link offset-delta chain, and a
separate reference-delta pack. Fifteen Git reference results were captured
and repeated on macOS and checked against the same portable corpus on Ubuntu.
The production reader now applies declared Latin-1 and the invalid UTF-8
fallback. A supplemental test verifies that a declared Latin-1 encoding wins
even if the bytes form valid UTF-8. The regular repository entry point now
opens the tested absolute alternate. Other named encodings and relative
alternate paths remain outside this verified scope.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,854 regular tests
and subtests across 13 packages. All 18 previous process comparison groups
passed. The actual checkout still produces `7.0.0-alpha.1+739` with all 33
JSON values and types matching C#. Ubuntu uses a snapshot of the checkout.

The verified inventory reaches 1,154 of 1,516 unit and component methods
(76.12%). Another 211 methods are needed for 90%. Windows validation is pending.

`CSHARP-GIT-OBJECT-READERS-EVIDENCE.json` records source and executable hashes,
case counts, process reports, and the checkout snapshot.
