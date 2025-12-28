# TODO: Icon Aliases Generator

## GitHub Actions AI Integration

Consider using GitHub Models API for free AI generation in GitHub Actions:

- **GitHub Models** provides access to GPT-4o, Claude, and other LLMs directly in Actions
- Free tier available for public repos with rate limits
- Uses `GITHUB_TOKEN` for authentication (no separate API key needed)
- Endpoint: `https://models.github.ai/inference`
- Docs: https://docs.github.com/en/github-models

### Implementation Notes

1. Add `--github-models` flag as alternative to `--openrouter`
2. Use `GITHUB_TOKEN` environment variable (automatically available in Actions)
3. API is OpenAI-compatible, so can reuse most of the request/response code
4. Supported models include: `gpt-4o`, `gpt-4o-mini`, `claude-3.5-sonnet`

### Example Action Workflow

```yaml
- name: Generate AI Knowledge Base
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    cd contrib/generate-icon-aliases
    go run . --generate-kb --github-models
```

This would make KB generation essentially free for the project's CI/CD pipeline.
