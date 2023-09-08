# goProbe legacy DB conversion tool

The `legacy` tool / binary is used to convert existing goProbe databases from a pre-v4.x format to the current database format. In the course of the conversion, size on disk is significantly reduced (by a factor of about 2-7 depending on the traffic pattern observed on the collecting host).

## Command line interface / usage
```bash
# legacy -h
Usage of legacy:
  -debug
    	Enable debug / verbose mode
  -dry-run
    	Perform a dry-run (default true)
  -i string
    	Path to (legacy) input goDB
  -l int
    	Custom LZ4 compression level (uses internal default if <= 0)
  -n int
    	Number of parallel conversion workers (default [[NUM_CPU/2]])
  -o string
    	Path to output goDB
  -p string
    	Permissions to use when writing files to DB (UNIX octal file mode) (default "644")
  -profile string
    	Path to output CPU profile
```
### Notes
- For safety reasons, the `legacy` tool does not perform / offer in-place conversion
- When `-debug` mode is enabled, a log message will be emitted for each converted daily directory (which may be a lot if the database is sufficiently large)
- Default file permissions for the output database are rather permissive (anybody can read), depending on the security requirements reducing permissions to e.g. `600` may be advisable.
- Appropriate directory (`+x`) permissions will automatically be applied to keep access consistent with the requested file permissions


## Examples
Perform a dry-run first (default) using four parallel workers (will perform all actions except for the actual writeout):
```bash
# legacy -n 4 -i /path/to/legacy/db -o /path/tp/output/db
```
If no errors are reported, perform the actual conversion:
```bash
# legacy -dry-run=false -n 4 -i /path/to/legacy/db -o /path/tp/output/db
```
After successful conversion, point goProbe / goQuery (>=v4.x) to the output database. Alternatively, delete / replace the legacy data with the converted one and update the existing configuration.