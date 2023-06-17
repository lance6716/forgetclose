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
