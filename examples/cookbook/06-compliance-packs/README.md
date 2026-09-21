# 06 — Compliance packs (HIPAA / PCI DSS / GDPR / Legal / SOC 2)

**Goal:** apply a ready-made detector/reaction preset to a tenant and audit
drift from it.

Prereq: [quickstart](../../quickstart/) is running, plus a presets directory.

## Presets

Example packs live in [`examples/compliance/presets/`](../../compliance/presets/).
Mount them and point the config at the directory:

```yaml
compliance:
  preset_dir: /etc/maskchain/presets
  enabled_packs: ["HIPAA", "PCI DSS", "GDPR"]   # empty = all found
```

A pack never defines new detectors — it references existing shield IDs:

```yaml
key: PCI DSS
name: Payment Card Industry Data Security Standard
rules:
  - detector_type: regex
    reaction: block
    masking: true
```

## Apply to a tenant

```bash
curl -s -X POST "http://localhost:9090/api/v1/tenants/default/compliance/apply" \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"pack": "PCI DSS"}'
```

## Audit a tenant against a pack

```bash
curl -s "http://localhost:9090/api/v1/tenants/default/compliance/report?pack=PCI%20DSS" \
  -H "Authorization: Bearer sk-test-default"
```

The report classifies each expected rule as **active**, **deviated**, or
**missing**; the UI shows the same as a deviation banner on the tenant page.

## Notes

- Mount the presets read-only and keep them versioned next to your config.
- Applying a pack updates the tenant's PII rules; review the diff in the
  tenant Activity tab.
