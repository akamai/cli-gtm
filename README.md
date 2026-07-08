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

If you want to compile it from source, you will need Go 1.26.4 or later:

1. Create a clone of the target repository:  
  `git clone https://github.com/akamai/cli-gtm.git`
2. Change to the package directory and compile the binary:
   - Linux/macOS/nix: `go build -o akamai-gtm`
   - Windows: `go build -o akamai-gtm.exe`

## Usage

```
$ akamai-gtm [--edgerc] [--section] <command> [sub-command]

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
$ akamai gtm update-datacenter --help
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
$ akamai gtm update-property --help
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

Property targets may be modified or added to properties by using the `target` argument. An example is provided in the following Examples section. The tool will modify the fields specified only. The target value is valid json. Misspelled field names will be ignored, possibly leading to an invalid target configuration. Valid fields are:

* datacenterId: int - Required
* weight: float64 - Required
* enabled: bool - Required
* servers: string list - Optional
* handoutCName: string - Optional

### query-status

```
$ akamai gtm query-status --help
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

would result in the following output:

```
Updating Datacenter(s) in domain example.akadns.net ... [OK]
example.akadns.net contains 1 properties
Updating Property: testproperty ... [OK]
testproperty contains 3 targets
Datacenter Update Summary

 Completed Updates
                     testproperty                ChangeId   5718f0f2-571a-419f-b6b8-e3479572fee3
 Failed Updates
                     No failed property updates
```

### Update traffic target in property

To disable a traffic target in a property:

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --disable
```

would result in the following output:

```
pLivenessTests:  []
testproperty contains 3 targets
Updating Traffic Targets ... [OK]

Response Status
 
ChangeId: 52e7d69f-027e-4500-af2b-e5672cb2dccc
```

To modify a property target's weight:

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --weight 20
```

would result in the following output:

```
pLivenessTests:  []
testproperty contains 3 targets
Updating Traffic Targets ... [OK]

Response Status
 
ChangeId: 379f34e9-d987-4455-8127-39b27227cbfe
```

To modify a property target's servers:

```
$ akamai gtm update-property example.akadns.net testproperty --datacenter 3131 --server 1.2.3.6 --server 1.2.1.1
```

would result in the following output:

```
pLivenessTests:  []
testproperty contains 3 targets
Updating Traffic Targets ... [OK]

Response Status
 
ChangeId: 2e7ffe58-8492-43fe-a2cc-ff24d36c899e

```

To modify (3131) and add (3134) property traffic targets:

```
$ akamai gtm update-property test_property.com.akadns.net targettest --dryrun --target '{"datacenterId": 3131,"weight":30,"enabled":true,"servers":["1.5.6.7"]}' --target '{"datacenterId": 3134,"weight":30,"enabled":true,"servers":["1.5.6.8"]}'
```

would result in the following output:

```
pLivenessTests:  []
test_property contains 3 targets
Updating Traffic Targets ... [OK]
Proposed Property Update
{
  "name": "targettest",
  "type": "weighted-round-robin",
  "ipv6": false,
  "scoreAggregationType": "mean",
  "stickinessBonusPercentage": 50,
  "useComputedTargets": false,
  "balanceByDownloadScore": false,
  "lastModified": "2026-07-08T08:03:17.360+00:00",
  "dynamicTTL": 300,
  "handoutLimit": 8,
  "handoutMode": "normal",
  "loadImbalancePercentage": 10,
  "ghostDemandReporting": false,
  "cname": "www.example.com",
  "trafficTargets": [
    {
      "datacenterId": 3131,
      "enabled": true,
      "weight": 30,
      "servers": [
        "1.5.6.7"
      ]
    },
    {
      "datacenterId": 3132,
      "enabled": true,
      "weight": 25,
      "servers": [
        "1.2.3.1"
      ],
    },
    {
      "datacenterId": 3133,
      "enabled": true,
      "weight": 45,
      "servers": [
        "1.1.3.1"
      ],
    },
    {
      "datacenterId": 3134,
      "enabled": true,
      "weight": 30,
      "servers": [
        "1.5.6.8"
      ]
    }
  ],
  "links": [
    {
      "rel": "self",
      "href": "https://<akamai-host>/config-gtm/v1/domains/test_property.com.akadns.net/properties/targettest"
    }
  ],
  "livenessTests": [
    {
      "name": "health check",
      "peerCertificateVerification": false,
      "testInterval": 60,
      "testObject": "/status",
      "httpError3xx": true,
      "httpError4xx": true,
      "httpError5xx": true,
      "httpMethod": "GET",
      "httpRequestBody": null,
      "disabled": true,
      "testObjectProtocol": "HTTP",
      "testObjectPort": 80,
      "pre2023SecurityPosture": false,
      "disableNonstandardPortWarning": false,
      "testTimeout": 25,
      "answersRequired": false,
      "recursionRequested": false,
      "alternateCACertificates": []
    }
  ]
}
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

would result in the following output:

```
pLivenessTests:  [test]
testproperty contains 3 targets
Updating Traffic Targets ... [OK]
livesness tests:  test
Processing liveness test:  test
Liveness test match!
pEnabled:  true
test.Disabled:  true

Response Status
 
ChangeId: f9d5f68c-dc67-4a60-8686-ec3fc642c8e9
```

### Query Status 

To query a domain's status:

```
$ akamai gtm query-status example.akadns.net 
```

would result in the following output:

```
Querying status ... [OK]
Collecting Domain status
Domain: example.akadns.net
Current Status [OK]

┌─────────────────────────┬──────────────────────────────────────────────────────────────────┐
│ ChangeId                │ 400b8b94-e112-4b72-baec-63c9b5247c47                             │
│ Message                 │ Current configuration has been propagated to all GTM nameservers │
│ Passing Validation      │ true                                                             │
│ Propagation Status      │ COMPLETE                                                         │
│ Propagation Status Date │ 2026-06-18T08:54:00.000+00:00                                    │
└─────────────────────────┴──────────────────────────────────────────────────────────────────┘

```

To query a datacenter's status:

```
$ akamai gtm query-status example.akadns.net --datacenter 3132
```

would result in the following output:

```
Querying status ... [OK]
Collecting DC status ... [OK]
Domain: example.akadns.net
Period Start: 2026-06-18T08:50:00Z
Period End: 2026-06-18T09:05:00Z

┌────────────┬───────────────────┬───────────────────────────┬───────────────┬─────────┬──────────┬────────┐
│ DATACENTER │     NICKNAME      │         TIMESTAMP         │   PROPERTY    │ ENABLED │ REQUESTS │ STATUS │
├────────────┼───────────────────┼───────────────────────────┼───────────────┼─────────┼──────────┼────────┤
│ 3132       │ property_test_dc2 │ 2026-07-08T19:36:15+05:30 │ testproperty  │ true    │ 0        │ 0      │
└────────────┴───────────────────┴───────────────────────────┴───────────────┴─────────┴──────────┴────────┘
```

To query a property's status:

```
$ akamai gtm query-status example.akadns.net --property testproperty
```

would result in the following output:

```
Querying status ... [OK]
Collecting Property status
Domain: example.akadns.net
Property: testproperty
Period Start: 2026-06-22T08:30:00Z
Period End: 2026-06-22T08:45:00Z
Status Summary Last Update: Not Available, Cutoff: 0

┌────────────┬──────────────┬─────────────┬─────────┬────────┬────────────────┬────────────────┬─────────────┬──────────────────┐
│ DATACENTER │ NICKNAME     │ TARGET NAME │ ENABLED │ WEIGHT │ TOTAL REQUESTS │ PROPERTY USAGE │ IP          │ STATE            │
├────────────┼──────────────┼─────────────┼─────────┼────────┼────────────────┼────────────────┼─────────────┼──────────────────┤
│ 3131       │ property_dc1 │             │ true    │ 40.0   │ 0              │ 0.00%          │ Score: 0.00 │ HandedOut: false │
│            │              │             │         │        │                │                │ Alive: false│                  │
│ 3132       │ property_dc2 │             │ true    │ 25.0   │ 0              │ 0.00%          │ Score: 0.00 │ HandedOut: false │
│            │              │             │         │        │                │                │ Alive: false│                  │
│ 3133       │ property_dc3 │             │ true    │ 45.0   │ 0              │ 0.00%          │ Score: 0.00 │ HandedOut: false │
│            │              │             │         │        │                │                │ Alive: false│                  │
└────────────┴──────────────┴─────────────┴─────────┴────────┴────────────────┴────────────────┴─────────────┴──────────────────┘

Datacenter Status

┌─────────────────────────────────────────┬─────────────────────┬─────────────────┐
│ TIMESTAMP                               │ DATACENTER NICKNAME │ REQUESTS STATUS │
├─────────────────────────────────────────┼─────────────────────┼─────────────────┤
│ No datacenter interval status available │                     │                 │
└─────────────────────────────────────────┴─────────────────────┴─────────────────┘

```

## License

This package is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fakamai%2Fcli-gtm?ref=badge_large)
