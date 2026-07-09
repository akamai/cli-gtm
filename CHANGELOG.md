# Release Notes

## Version 0.7.0 

### Features/Enhancements

* Upgrade to Edgegrid v13.3.0
* Update project license file
* Migrate to go 1.26.5
* Standardize command status/failure messaging ([OK/FAIL]) 
* Standardize table rendering format for enhanced readability.

## Version 0.6.0 (January 2026)

### Features/Enhancements

* Add support for linux-arm64 architecture
* Upgrade to Edgegrid v12.3.0
* Session based authentication
* Migrate to go 1.25
* Added - reportsgtm package
    - Provides traffic reporting for individual datacenters within a GTM domain
    - Supports property-level traffic reports, including data across multiple datacenters
    - Enables retrieval of IP availability for GTM properties

## Version 0.5.0 (May 10, 2023)

### Features/Enhancements

* Add M1 support
* Migrate to go 1.18

## Version 0.4.0

### Features/Enhancements

* Add support to property update to enable/disable liveness tests
* Change .edgerc section default to 'default'
* Add account-key as accountkey flag alias
* Update README

## Version 0.3.0 

### Features/Enhancements

* Add support for property update target modification
* Allow timeout duration override for property and datacenter update
* Allow dryrun execution for property and datacenter update
* Include target weight in status output
* Update build to use go modules
* Update README

## Version 0.2.0

### Features/Enhancements

Initial release

* Support target datacenter update across properties.
* Support target updates per property
* Support domain, property and datacenter status queries

