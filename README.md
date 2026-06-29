# AGW

## MVP Flow

```bash
agw config init --root /path/to/personal/agw-root
agw workspace new --root /path/to/personal/agw-root --id agw --name AGW --storage workspaces/github.com/kenfdev/agw --project agw=/path/to/agw:/workspace --service dev --workspace-root /workspace
agw workspace prepare agw --output prompt.md
agw workspace apply agw ./generated
agw build agw
agw up agw
agw attach agw
```
