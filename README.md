# tf2vpk

Libraries and tools for working with Titanfall 2 VPKs.

[`Download`](https://nightly.link/pg9182/tf2vpk/workflows/ci/master?preview)

### Features

- Command-line utilities.
- Extremely memory and CPU-efficient.
- Can run anywhere Go can run, without a C compiler (with a performance penalty).
- Tools to optimize VPKs without recompressing from scratch and preserving all metadata.
- Full-featured VPK file listing.
- Go library exposing most functionality.
- Designed for r2 VPKs, should work with r1/r5 as well.
- Extremely flexible.
- Deterministic output for most commands.
- Supports unpacking VPKs with full support for load/texture flags (it can generate either an optimized flags file with directory-based inheritance, or it can have one entry for every file in the source VPK).
- WIP: Supports repacking VPKs with full support for ignoring files and setting load/texture flags (as either a new VPK, an update of an existing one, or an addition to an existing one), optionally reusing chunks where possible.

### Examples

#### List a VPK

Any of the following commands will show a simple list of all files in the VPK.

```
tf2-vpklist /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000
```

```
tf2-vpklist /path/to/Titanfall2/vpk/client_mp_angel_city.bsp.pak000_000.vpk
```

```
tf2-vpklist /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk
```

#### Create a new unpacked VPK

The following command will create a basic `.vpkflags` and `.vpkignore` file in the provided directory so it can be repacked later.

```
tf2-vpkunpack /path/to/empty/folder/for/vpk
```

#### Extract a single VPK to the current directory

Any of the following commands will extract the provided vpk into the provided directory, including a pre-configured `.vpkflags` and `.vpkignore` so it can be repacked later, preserving flags.

```
tf2-vpkunpack /path/to/empty/folder/for/vpk /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000
```

```
tf2-vpkunpack /path/to/empty/folder/for/vpk /path/to/Titanfall2/vpk/client_mp_angel_city.bsp.pak000_000.vpk
```

```
tf2-vpkunpack /path/to/empty/folder/for/vpk /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk
```

#### Extract models from all VPKs

The following command will extract models from all VPKs, using tar.

```
for vpk in /path/to/Titanfall2/vpk/english*_dir.vpk
do
    tf2-vpk2tar "$vpk"  --exclude '/*' --include '/models' | tar xvf -
done
```

The `tf2-vpk2tar` command is a better option than `tf2-vpkunpack` if you don't care about repacking the VPK later and want to have finer-grained control over the output or integrate it with other tools.

#### Convert a VPK to SquashFS (Linux)

The following command directly converts a VPK to SquashFS so it can be mounted.

```
tf2-vpk2tar /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk | sqfstar client_mp_angel_city.bsp.pak000.squashfs
```

#### Optimize and remove unnecessary files from VPKs

The following command optimizes VPKs by removing unused files and regenerating the blocks.

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

Remember to copy the `enable.txt` file from the original vpk folder to the new one, or the game won't work.

#### Optimize and remove unnecessary files from VPKs for the Northstar dedicated server

The following command also removes large files only used by the client.

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '*.vtf' \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

Remember to copy the `enable.txt` file from the original vpk folder to the new one, or the game won't work.

#### List a VPK verbosely and verify checksums

The following command verbosely lists all metadata from the VPK in human-readable form, while also reading and verifying the contents.

```
tf2-vpklist -lhft /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk
```

```
000 00000000000100000000000100000001 0000000000001000 A77AB754  81.18 %    4.6 kB    5.6 kB  materials/models/domestic/luggage_01_screen_col.vtf               OK # load=[00:VISIBLE 08:CACHE 20:TEXTURE_UNK2] texture=[03:DEFAULT]
000 00000000000100000000000100000001 0000000000001000 C2B89C0A  13.00 %   11.4 kB   87.7 kB  materials/models/humans/mri/subtle_scanline.vtf                   OK # load=[00:VISIBLE 08:CACHE 20:TEXTURE_UNK2] texture=[03:DEFAULT]
000 00000000000011000000000100000001 0000000000001000 7CB126EE  62.00 %  109.7 kB  176.9 kB  materials/world/cloud_masks/mp_angel_city_mask01_col.vtf          OK # load=[00:VISIBLE 08:CACHE 18:TEXTURE_UNK0 19:TEXTURE_UNK1] texture=[03:DEFAULT]
000 00000000000000000000000100000001 0000000000000000 56AEEFAC  65.45 %    180  B    275  B  materials/tools/toolsblock_los_n_clip.vmt                         OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 BF7AB187  69.89 %    253  B    362  B  materials/particle/smoke/smoke_puff_01_db128_nc2_nodoftsaa.vmt    OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 B25FE559  65.27 %    312  B    478  B  materials/models/domestic/luggage_01_screen.vmt                   OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 ED873D51  69.37 %    188  B    271  B  resource/overviews/mp_angel_city.txt                              OK # load=[00:VISIBLE 08:CACHE] texture=[]
006 00000000000000000000000100000001 0000000000000000 C7153AF9  62.04 %    201  B    324  B  scripts/levels/mp_angel_city.rson                                 OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 1F294071  51.54 %   50.7 kB   98.3 kB  materials/correction/mp_angel_city_dlc.raw                        OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 3052F29E  27.23 %    8.7 kB   32.0 kB  scripts/vscripts/client/cl_carrier.gnut                           OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 9D8E4973  85.79 %    157  B    183  B  scripts/vscripts/client/levels/cl_mp_angel_city.nut               OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 8FA3B7AC  26.12 %    672  B    2.6 kB  models/water/water_puddle_decal_01.mdl                            OK # load=[00:VISIBLE 08:CACHE] texture=[]
000 00000000000000000000000100000001 0000000000000000 FB46E776  28.99 %    754  B    2.6 kB  models/signs/market/angel_city_market_sign18.mdl                  OK # load=[00:VISIBLE 08:CACHE] texture=[]
```

### Building

Pre-built binaries for the latest commit can be found [here](https://nightly.link/pg9182/tf2vpk/workflows/ci/master?preview).

Note that building with `CGO_ENABLED=1` results in a significant performance
impact; tf2-vpk2tar is about 4-8x slower (even with parallelism).

#### Build with a C compiler

```
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpkoptim
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpklist
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpkunpack
```

#### Build without a C compiler

```
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpkoptim
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpklist
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpkunpack
```

#### Build for Windows using MinGW

```
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpkoptim
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpklist
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpkunpack
```

#### Build for Windows without using MinGW

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpkoptim
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpklist
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpkunpack
```

### VPK unpacking/packing

TODO: more specific documentation
