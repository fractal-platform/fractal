## Fractal 
go version for fractal blockchain

[![TravisCI](https://travis-ci.org/fractal-platform/fractal.svg?branch=master)](https://travis-ci.org/fractal-platform/fractal)

## Develop Steps (for new features & bugs)
1. create a task in jira(http://10.1.1.11:9080/projects/FTL/summary )，you can get a task id (e.g FTL-1)  
2. create a git branch, branch name should be associated with task id (e.g FTL-1)  
3. change task status to In Progress in jira  
4. work on the new branch, and commit your code  
5. create a merge request in gitlab, and change task status to Done in jira  
6. code review, and merge the new branch to master  

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
    git clone http://10.1.1.11:8000/fractal/fractal.git
    ```
    
3. build tools
    ```
    cd $GOPATH/src/fractal
    go install -v -ldflags "-X main.gitCommit=$(git log --pretty=format:'%h' -1)" ./cmd/gftl/  
    go install -v -ldflags "-X main.gitCommit=$(git log --pretty=format:'%h' -1)" ./cmd/gtool/
    sudo cp transaction/txec/libwasmlib.dylib /usr/local/lib/
    ```

## Setup Basic Test Environment
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

### About test.toml
see [cmd/gftl/test.toml](cmd/gftl/test.toml)

### About NAT
Add param while starting gftl:
```
--nat any 
```

## Send Transaction & Query Account  
### Simple Transfer  
1. Send a transaction to transfer value
    ```
    gtool tx --rpc http://127.0.0.1:8545 --to 0xXXXX --chainid 999 --keys data1/keys --pass 666 send
    ```  
    
2. Lookup account balance
    ```
    gtool state --rpc http://127.0.0.1:8545 --addr 0xXXXX account
    ```  
    

### Contract Deploy & Call  
1. Prepare contract wasm&abi file  
2. Deploy contract, and get the contract address
    ```
    gtool tx --rpc http://127.0.0.1:8545 --chainid 999 --keys data1/keys --pass 666 --wasm ./addressbook.wasm deploy
    ``` 
    
3. Call contract(set the contract address for the arg "to")
    ```  
    gtool tx --rpc http://127.0.0.1:8545 --chainid 999 --keys data1/keys --pass 666 --to 0xXXXX --abi addressbook.abi --action insert --args '["1234"]' call
    ```  
    
4. Query contract storage(set the contract address for the arg "addr")  
    ```
    gtool state --rpc http://127.0.0.1:8545 --addr 0xXXXX --table person --skey <skey> storage
    ```  
    The skey param must be constructed manually. For the addressbook contract, the skey is 0x14XXXX, where XXXX is the caller's address.
  
