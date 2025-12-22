# gswarm.json Configuration Reference

The `gswarm.json` file is used within templates and agent directories to configure how a Gemini Swarm agent is provisioned and executed.

## Fields

### `image` (string)
The container image to use for the agent.
- **Default**: `gemini-cli-sandbox`
- **Example**: `"image": "my-custom-gemini-agent:latest"`

### `detached` (boolean)
Whether the agent should run in detached mode by default.
- **Default**: `true`
- **Example**: `"detached": false`

### `use_tmux` (boolean)
*Note: This feature is currently in design/implementation.*

If set to `true`, the agent's main process will be wrapped in a `tmux` session. This enables persistent interactive sessions that can be detached and re-attached using the `gswarm attach` command.
- **Default**: `false`
- **Details**: When enabled, `gswarm` will attempt to use a version of the configured image with a `:tmux` tag if available.
- **Example**: `"use_tmux": true`

## Inheritance
`gswarm` uses a template inheritance system. Configuration fields are merged from the `default` template, then the specified template type, and finally any overrides in the agent's own directory. The last value defined for a field takes precedence.
