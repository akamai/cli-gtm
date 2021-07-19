# Akamai CLI for Global Traffic Management (GTM)

[![Go Report Card](https://goreportcard.com/badge/github.com/akamai/cli-gtm)](https://goreportcard.com/report/github.com/akamai/cli-gtm) [![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm?ref=badge_shield)

An [Akamai CLI](https://developer.akamai.com/cli) package for managing GTM Domains and associated objects.

## Getting Started

### Installing

To install this package, use Akamai CLI:

```sh
$ akamai install gtm
```

You may also use this as a stand-alone command by downloading the
[latest release binary](https://github.com/akamai/cli-gtm/releases)
for your system, or by cloning this repository and compiling it yourself.

### Compiling from Source

If you want to compile it from source, you will need Go 1.14 or later.

1. Fetch the package:  
  `go get github.com/akamai/cli-gtm`
2. Change to the package directory:  
  `cd $GOPATH/src/github.com/akamai/cli-gtm`
3. Compile the binary:
   - Linux/macOS/nix: `go build -o akamai-gtm`
   - Windows: `go build -o akamai-gtm.exe`
4. Move the binary (`akamai-gtm` or `akamai-gtm.exe`) in to your `PATH`

## Usage

```
  akamai-gtm [--edgerc] [--section] <command> [sub-command]

Description:
   Manage GTM Domains and assoc objects

Global Flags:
   --edgerc value      Location of the credentials file (default: "/home/testuser/.edgerc") [$AKAMAI_EDGERC]
   --section value     Section of the credentials file (default: "gtm") [$AKAMAI_EDGERC_SECTION]

Built-In Commands:
  update-datacenter
  update-property
  query-status
  list
  help
```

### update-datacenter

```
$ akamai gtm update-datacenter -help
Name:
   akamai-gtm update-datacenter

Description:
   Update datacenter configuration

Usage:
   akamai-gtm update-datacenter <domain> [--datacenter] [--enable] [--disable] [--verbose] [--json] [--complete] [--timeout] [--dryrun]

Flags:
   --datacenter value      Apply change to specified datacenter traffic target in all property references by id or nickname.
   --enable                Enable specified datacenter traffic target(s) in all property references.
   --disable               Disable specified datacenter traffic target(s) in all property references.
   --verbose               Display verbose result status.
   --json                  Return status in JSON format.
   --complete              Wait for change completion.
   --timeout value         Change completion wait timeout in seconds. (default: 300)
   --dryrun                Return planned datacenter traffic target change(s).
```

### update-property

```
$ akamai gtm update-property -help
Name:
   akamai-gtm update-property

Description:
   Update property configuration

Usage:
   akamai-gtm update-property [domain, property] [--datacenter] [--liveness_test] [--enable] [--disable] [--weight] [--target] [--server] [--verbose] [--json] [--complete] [--timeout] [--dryrun]

Flags:
   --datacenter value      Apply change to specified datacenter traffic target by id or nickname. Multiple datacenters may be specified.
   --liveness_test value   Apply change to specified liveness test. Multiple liveness tests may be specified.
   --enable                Enable specified datacenter traffic target or property liveness_test.
   --disable               Disable specified datacenter traffic target or property liveness_test.
   --weight value          Apply 'weight' to specified datacenter traffic target. (default: 0)
   --target value          Update specified target field values or add target if doesn't exist. Multiple target flags may be specified.
   --server value          Update server for specified datacenter traffic target. Multiple server flags may be specified.
   --verbose               Display verbose result status.
   --json                  Return status in JSON format.
   --complete              Wait for change completion.
   --timeout value         Change completion wait timeout in seconds. (default: 300)
   --dryrun                Return planned property change(s).
```

#### Target modifications

Property targets may be modified or added to properties by using the `target` argument. An example is provided in the following Examples section. The tool will modify the fields specified only. The target value is valid json. Mispselled field names will be ignored, possibly leading to and invalid target configuration. Valid fields are:

* datacenterId: int - Required
* weight: float64 - Required
* enabled: bool - Required
* servers: string list - Optional
* name: string - Optional
* handoutCName: string - Optional

### query-status

```
$ akamai gtm query-status -help
Name:
   akamai-gtm query-status

Description:
   Query current status of domain, property or datacenter

Usage:
   akamai-gtm query-status <domain> [--datacenter] [--property] [--verbose] [--json]

Flags:
   --datacenter value  Report status of specified datacenter by id or nickname.
   --property value        Report status of specified property.
   --verbose               Display verbose status.
   --json                  Return status in JSON format.
```

## Examples

### Enable datacenters in domain

To enable one or more datacenters references in all property targets:

```
$ akamai gtm update-datacenter example.akadns.net --datacenter 3131 --datacenter 3132 --enable
```

### Update traffic target in property

To enable a traffic target in a property:

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --disable
```

To modify a property target's weight:                                    

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --weight 20
```

To modify a property target's servers:

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --server 1.2.3.6 --server 1.2.1.1
```

To modify (3131) and add (3134) property traffic targets:

```
$ akamai gtm update-property test_property.com.akadns.net targettest --dryrun -target '{"datacenterId": 3131,"weight":30,"enabled":true,"servers":["1.5.6.7"]}' -target '{"datacenterId": 3134,"weight":30,"enabled":true,"servers":["1.5.6.8"]}'
```

Note: On Windows platforms, use double quotes to enclose the target JSON string and escape the double quotes within the JSON. E.g.

```
C:\temp> akamai-gtm.exe --edgerc c:\scratch\.edgerc --section default update-property test_property.com.akadns.net targettest --target "{\"datacenterId\": 3131,\"weight\":0,\"enabled\":true}" --target "{\"datacenterId\": 3134,\"weight\":0,\"enabled\":true}"
```

Note the dryrun directive in the command line.

To disable a liveness test in a property:

```
$ akamai gtm update-property example.akadns.net testproperty --liveness_test test --disable
```

### Query Status 

Query a datacenter's status:

```
$ akamai gtm query-status example.akadns.net --datacenter 3132
```

To query a property's status:

```
$ akamai gtm query-status example.akadns.net --property testproperty
```

## License

This package is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm?ref=badge_large)
