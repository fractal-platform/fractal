## Fractal 
go version for fractal blockchain

master | dev
-------|----------
[![TravisCI](https://travis-ci.org/fractal-platform/fractal.svg?branch=master)](https://travis-ci.org/fractal-platform/fractal) | [![TravisCI](https://travis-ci.org/fractal-platform/fractal.svg?branch=dev)](https://travis-ci.org/fractal-platform/fractal)

## Build Steps
1. make your own go env, set $GOPATH, and add $GOPATH/bin to PATH env
    ```
    cd ~
    mkdir -p go/bin go/src
    export GOPATH=~/go
    export PATH=$PATH:$GOPATH/bin
    ```
    
2. clone source
    ```
    cd $GOPATH/src
    git clone git@github.com:fractal-platform/fractal.git github.com/fractal-platform/fractal
    ```
    
3. build
    ```
    cd $GOPATH/src/github.com/fractal-platform/fractal
    go install -v -ldflags "-X main.gitCommit=$(git log --pretty=format:'%h' -1)" ./cmd/gftl/  
    go install -v -ldflags "-X main.gitCommit=$(git log --pretty=format:'%h' -1)" ./cmd/gtool/
    sudo cp transaction/txec/libwasmlib.dylib /usr/local/lib/
    ```

## Running Fractal blockchain
### Setup Basic Test Environment
1. Setup keys&genesis
    ```
    cd ~
    mkdir test
    gtool keys --keys data1/keys --pass 666 newkeys
    gtool keys --keys data2/keys --pass 666 newkeys
    address1=$(gtool keys --keys data1/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
    gtool gstate --pass 666 --packerKeyOwner $address1 gen
    ```

2. Setup the first node
    ```
    cd ~/test
    cp $GOPATH/src/fractal/cmd/gftl/test.toml .
    gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data1 --port 30303 --pprof --pprofport 6060 --verbosity 3 --mine --packer --unlock 666
    ```
  
3. Setup the second node, start a new terminal and exec:
    ```
    cd ~/test
    enode1=$(gtool admin --rpc http://127.0.0.1:8545 enode)
    gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8546 --datadir data2 --port 30304 --pprof --pprofport 6061 --verbosity 3 --mine --packer --unlock 666 --bootnodes $enode1  
    ```

4. Test, start a new terminal and exec:
```
cd ~/test
address1=$(gtool keys --keys data1/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
address2=$(gtool keys --keys data2/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
gtool tx --rpc http://127.0.0.1:8545 --to $address2 --chainid 999 --keys data1/keys --pass 666 send
```

### Setup Packer
1. Add(Modify) a Packer
    ```
    cd ~/test
    cp $GOPATH/src/fractal/cmd/gftl/packer_keys.abi .
    address1=$(gtool keys --keys data1/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
    packerpubkey1=$(gtool keys --keys data1/keys --pass 666 list | grep "Packer Public Key" | awk -F: '{print $2}')
    gtool packer --rpc http://127.0.0.1:8545 --chainid 999 --keys data1/keys --pass 666 --abi packer_keys.abi --packerId 0 --packerAddress http://127.0.0.1:8545 --packerCoinbase $address1 --packerPubKey $packerpubkey1 setPacker
    ```
    
2. Send transaction
    ```
    cd ~/test
    address1=$(gtool keys --keys data1/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
    address2=$(gtool keys --keys data2/keys --pass 666 list | grep "Account Key" | awk -F: '{print $2}')
    gtool tx --rpc http://127.0.0.1:8545 --to $address2 --chainid 999 --keys data1/keys --pass 666 --packer send
    ```

