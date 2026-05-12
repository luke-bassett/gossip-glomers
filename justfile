alias ch1 := echo
alias ch2 := generate

build-echo:
    cd maelstrom-echo && go build -o bin/maelstrom-echo

echo: build-echo
    ./maelstrom/maelstrom test -w echo --bin ./maelstrom-echo/bin/maelstrom-echo --node-count 1 --time-limit 10

build-generate:
    cd maelstrom-generate && go build -o bin/maelstrom-generate
    
generate: build-generate
    ./maelstrom/maelstrom test -w unique-ids --bin ./maelstrom-generate/bin/maelstrom-generate --node-count 3 --time-limit 10

serve:
    ./maelstrom/maelstrom serve
