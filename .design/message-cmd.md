# Message command

This would be a new command

`scion message` (msg for short)

That would send a message to harnesses which enqueues new messages to an agent.

This would require tmux and use the 'send-keys' command of tmux combined with the 'exec' command of the runtime

It would send the message plus the "Enter" special tmux key

There is an optional -i (--interrupt) flag that first uses send-keys to send either 'Escape' or 'C-c' depending on the harness (this is harness specific, 'C-c' for generic)

There should be a --b or --broadcast flag, which sends the message to all agents who are running.