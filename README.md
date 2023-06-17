# `forgetclose` is a linter that reminds you to `Close` variables.

## Usage

```shell
make
```

You can find the binary in `bin/forgetclose`. Then at your project root, run:

```shell
forgetclose -type "${package}.${type}" ./...
```

For example, if you want to check `*sql.Rows` is closed:

```shell
forgetclose -type "database/sql.Rows" ./...
```

The first run will be slow because it needs golang to build SSA.

## Supported patterns

You can see them under `pkg/analyzer/testdata/src/testcase`.

## Help this project

### Development

- Support cross package check
- Add leak information in the error message
- Speed up the analysis
- Support check multiple types in one run

A short guide to contribute:

1. Change the test case in `pkg/analyzer/testdata/src/testcase`. 
If it's a single package test, you can change the `pkg/analyzer/testdata/src/testcase/wip/plain.go` file.
If a line should be reported by linter, add a comment `// want "${message regexp patterm}"` at the end of the line.
2. Change the test file `pkg/analyzer/analyzer_test.go` to enable the test case, like uncomment the line 
`analysistest.Run(t, testdata, checker, "wip")`.
3. Make your changes.
4. Move the test case into `pkg/analyzer/testdata/src/testcase` or create another test package.
Change the test file `pkg/analyzer/analyzer_test.go` to test them all.
5. Open a PR. Thanks in advance!

And you may find the constant `isDebug` in `pkg/analyzer/analyzer.go` helpful.

### Test

- Open an issue of realistic usages that linter can't detect

### Maintenance

- Add CI to this project
- Refine the README.md