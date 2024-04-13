# tf2vpk
Libraries and tools for working with Titanfall 2 VPKs.

### Examples

#### List a VPK and verify checksums:

```
tf2-vpk2tar /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000 --test --verbose
```

```
tf2-vpk2tar /path/to/Titanfall2/vpk/client_mp_angel_city.bsp.pak000_000.vpk --test --verbose
```

```
tf2-vpk2tar /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk --test --verbose
```

#### Extract a single VPK to the current directory:

```
tf2-vpk2tar /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000 | tar xvf -
```

```
tf2-vpk2tar /path/to/Titanfall2/vpk/client_mp_angel_city.bsp.pak000_000.vpk | tar xvf -
```

```
tf2-vpk2tar /path/to/Titanfall2/vpk/englishclient_mp_angel_city.bsp.pak000_dir.vpk | tar xvf -
```

#### Extract models from all VPKs:

```
for vpk in /path/to/Titanfall2/vpk/english*_dir.vpk
do
    tf2-vpk2tar "$vpk"  --exclude '/*' --include '/models' | tar xvf -
done
```

#### Optimize and remove unnecessary files from VPKs:

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

#### Optimize and remove unnecessary files from VPKs for the Northstar dedicated server:

```
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '*.vtf' \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

### Building

Note that building with `CGO_ENABLED=1` results in a significant performance
impact; tf2-vpk2tar is about 4-8x slower (even with parallelism).

#### Build with a C compiler:

```
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

#### Build without a C compiler:

```
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

#### Build for Windows using MinGW:

```
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

#### Build for Windows without using MinGW:

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```
