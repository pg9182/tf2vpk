# tf2vpk
Libraries and tools for working with Titanfall 2 VPKs.

## Examples

**Optimize and remove unnecessary files from VPKs:**

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

**Optimize and remove unnecessary files from VPKs for the Northstar dedicated server:**

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '*.vtf' \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```
