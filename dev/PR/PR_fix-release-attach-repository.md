## Summary

- Pass the explicit GitHub repository to `gh release upload` in the attach job.
- Add a regression test for release workflows that upload without a checkout.

## Testing

- `make validate`
- `make test-race`
- Release workflow YAML parsing

## Safety and compatibility

- The change only affects release asset attachment.
- Existing same-name release assets remain replaceable through `--clobber`.

## Review notes

- The original build matrix and artifact downloads succeeded; only repository
  inference in the checkout-free attach job failed.
