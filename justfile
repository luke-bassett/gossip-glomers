alias ch1 := echo
alias ch2 := generate
alias ch3 := broadcast

build-echo:
    cd maelstrom-echo && go build -o bin/maelstrom-echo

echo: build-echo
    ./maelstrom/maelstrom test -w echo --bin ./maelstrom-echo/bin/maelstrom-echo --node-count 5 --time-limit 10

build-generate:
    cd maelstrom-generate && go build -o bin/maelstrom-generate
    
generate: build-generate
    ./maelstrom/maelstrom test -w unique-ids --bin ./maelstrom-generate/bin/maelstrom-generate --node-count 5 --time-limit 10

build-broadcast:
    cd maelstrom-broadcast && go build -o bin/maelstrom-broadcast

broadcast rate="10" time-limit="20": build-broadcast
    ./maelstrom/maelstrom test -w broadcast --bin ./maelstrom-broadcast/bin/maelstrom-broadcast --node-count 1 --time-limit {{time-limit}} --rate {{rate}} 



serve:
    ./maelstrom/maelstrom serve
