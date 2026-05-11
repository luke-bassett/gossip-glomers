alias ch1 := echo

build-echo:
    cd maelstrom-echo && go build -o bin/maelstrom-echo

echo: build-echo
    ./maelstrom/maelstrom test -w echo --bin ./maelstrom-echo/bin/maelstrom-echo --node-count 1 --time-limit 10

serve:
    ./maelstrom/maelstrom serve
