I ran into an issue, when I wanted to override
the container image in an agent, from:

gemini-cli-sandbox:tmux
to 
gemini-cli-sandbox:golang

I did not realize that using tmux would alway force the tag to :tmux

will have to consider the trade-off of an agent wanting to specify a image
identifier with a tag, which still has tmux installed

