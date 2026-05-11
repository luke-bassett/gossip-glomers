alias ch1 := echo

echo:
    ./maelstrom/maelstrom test -w echo --bin ./maelstrom-echo/maelstrom-echo --node-count 1 --time-limit 10

serve:
    ./maelstrom/maelstrom serve
