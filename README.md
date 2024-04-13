# tf2vpk
Libraries and tools for working with Titanfall 2 VPKs.

## Examples

**List a VPK and verify checksums:**

```bash
tf2-vpk2tar /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000 --test --verbose
```

**Extract a single VPK to the current directory:**

```bash
tf2-vpk2tar /path/to/Titanfall2/vpk client_mp_angel_city.bsp.pak000 | tar xvf -
```

**Extract models from all VPKs:**

```bash
for x in /path/to/Titanfall2/vpk/*_000.vpk
do
    x=${x##*/}
    x=${x%_000.vpk}
    tf2-vpk2tar /path/to/Titanfall2/vpk "$x"  --exclude '/*' --include '/models' | tar xvf -
done
```

**Optimize and remove unnecessary files from VPKs:**

```bash
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

**Optimize and remove unnecessary files from VPKs for the Northstar dedicated server:**

```bash
tf2-vpkoptim /path/to/Titanfall2/vpk \
    --verbose \
    --merge \
    --exclude '*.vtf' \
    --exclude '/depot' \
    --exclude '*.bsp_lump' \
    --output /path/to/new/vpks
```

## Building

**Build with a C compiler:**

```bash
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

**Build without a C compiler:**

```bash
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

**Build for Windows using MinGW:**

```bash
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```

**Build for Windows without using MinGW:**

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpk2tar
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -v -x ./cmd/tf2-vpkoptim
```
