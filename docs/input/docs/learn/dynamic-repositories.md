---
Order: 20
Title: Dynamic Repositories
RedirectFrom: docs/more-info/dynamic-repositories
---

GitVersion.exe requires access to the Git repository in order to do / infer all
the lovely things that it does.

Well maybe that's not so much of a revelation..

_But did you know_ that in some circumstances, you may find that when your code
is built, the build process isn't aware of the Git repository at all - i.e
_there is no Git Repository available locally to the build!_

For an example of one such circumstance, you can have a read about
[Team City's checkout mode: Automatically on Server][tc-checkout].

So `how is GitVersion meant to work in that scenario?` - well it needs to be
able to obtain a copy of the Git repo on the fly and not rely on the build
system making one locally available..

Enter: **Dynamically Obtained Repo's**

## Tell GitVersion.exe how to obtain your repository

Unless you tell GitVersion.exe how to obtain the Git repository, it will assume
that the Git repository is already available locally to the process - i.e. it
will assume there is already a ".git" folder present, and it will use it.

To tell GitVersion.exe to obtain the repository on the fly, you need to call
`GitVersion.exe` with the following arguments:

* `--url [the url of your git repo]`
* `--username [authentication username]` (short alias: `-u`)
* `--password [authentication password]` (short alias: `-p`)
* `--branch [branch name]` (short alias: `-b`)
* `--commit [commit id]`

Long-form arguments are recommended for scripts and documentation.

Please note that these arguments are described when calling `GitVersion.exe --help`.

Pass `--branch` to select the remote branch to version. If it is omitted,
GitVersion requires a branch from the build agent or an environment-provided
branch context.

NB: In Team City, the VCS branch is available from a configuration parameter
with the following name:

`teamcity.build.vcs.branch.<VCS root ID>`

Where `<VCS root ID>` is the `VCS root ID` as described on the Configuring
`VCS Roots` page.

## Where is the repository stored

GitVersion stores the dynamic repository in the system temp directory and reuses
it across builds. Use `--dynamic-repo-location` to choose a different location.

## Partial clones

With `--url`, GitVersion defaults to a bare partial clone for advertised branches.
It calculates the version without checking out repository files:

```console
gitversion --url https://github.com/example/project.git --branch feature/example
```

GitVersion keeps commit history, tags available through the fetched branches, and
remote branch references for version calculation. If the effective configuration
has `ignore.paths` entries, it uses `blob:none` to keep trees for changed-path
detection without downloading file contents. Otherwise it uses `tree:0` to omit
trees as well. Configuration comes from the calling directory or the explicit
`--config` file.

Servers that do not advertise filtering can still download all objects without
checking out files. Pass `--no-partial-clone` to use the original full clone and
checkout, for example if a server rejects the requested filter or a script relies
on checked-out files. Canonical refs outside `refs/heads/*`, such as pull request
refs, use the original preparation path.

Each mode has a separate cache. The partial caches use the `-treeless` suffix for
`tree:0` and `-lightweight` for `blob:none`. The `--no-fetch` option skips updates
to an existing cache. A fresh clone still downloads history.

[tc-checkout]: https://confluence.jetbrains.com/display/TCD7/VCS+Checkout+Mode
