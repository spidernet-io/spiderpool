name: Documentation issue
description: help us improve doc
labels: kind/doc
body:
  - type: dropdown
    id: opt-doc
    attributes:
      label: The Type of Document Issue
    multiple: true
    options:
      - "Lacking"
      - "Wrong"
      - "outdated"
      - "Other"
    validations:
      required: true
  - type: textarea
    id: doc
    attributes:
      label: What's wrong with this document?
    placeholder: |
      the document is outdated
    validations:
      required: true
  - type: input
    id: doc-path
    attributes:
      label: Document Path Or Link
      description: |
        [e.g. 0.13.9, 0.12.0]
    validations:
      required: true
