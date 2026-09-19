# Design: `typ file processor` tool

- **Date:** 2026-09-19
- **Status:** Proposal

## 1. Purpose

Given a Typst file, this tool does a number of things, called Features.
Features can be invoked individually by the tool's parameters. If no
feature is specified, it defaults to invoke all features.

## 2. Features

### 2.1 Extract References

Step 1 - Extract References and Add to the Bibliography file: 
The Typst file should be in 'KnowledgeStore'. It extracts references from the Typst file, 
add them to '/references/references.bib'. References are listed in the `References` section,
which can be the root level section (i.e., `= References`) or a subsection (such as 
`== References`).

If a reference is already in the bib, do not add it.

Step 2 - Use Typst reference syntax:
It replaces the use of the references in the Typst file using the Typst syntax.

Step 3 - Remove the `References` section.

Examples:
```text
...
Jev evaluates every question independently against the same supplied state. Users can 
supply multiple questions with the same context. Jev answers them all in one call ([TypeSafe AI][1]).
...
Jev interprets the supplied information using knowledge encoded in its model parameters, much as an LLM 
understands the semantics of text. But Jev's job is *not knowledge retrieval*. Its job is to make a fast 
judgment over the state you supplied. TypeSafe explicitly characterizes good Jev questions as judgments 
that a knowledgeable person could make quickly *“given the right context.”* ([TypeSafe AI][1])
...
That fits remarkably well with the *candidate → investigative exploration* architecture we 
discussed for SemOS: conventional retrieval finds candidates; Jev can cheaply perform many of 
the fuzzy judgments that determine *which candidates to keep and where to 
explore next*. ([TypeSafe AI][2])
...
= References
[1]: https://docs.typesafe.ai/introduction "Introduction - TypeSafe AI"
[2]: https://typesafe.ai/blog/introducing-system-one-models-and-jev?utm_source=chatgpt.com "Introducing System One Models and Jev - TypeSafe AI Blog"
```

#### 2.1.1 Extract References and Add to references.bib
This file has two references, listed in `= References`. They should be extracted and added to `/references/references.bib`
(i.e., in the file 'KnowledgeStore/references/references.bib'):

```text
@online{introducing-system-one-models-and-jev-typesafe-ai-blob,
  title = {Introducing System One Models and Jev - TypeSafe AI Blog},
  url = {https://typesafe.ai/blog/introducing-system-one-models-and-jev?utm_source=chatgpt.com},
}

...

@online{introduction-typesafe-ai,
  title = {Introduction - Typesafe AI},
  url = {https://docs.typesafe.ai/introduction},
}
```

The format of entries in 'references.bib':
```text
@online{<slug-of-title>,
  title = {string},
  url = {the-url},
}
```

Note that:
- Entries in '/references/references.bib' are sorted by reference slugs.
- Entries in '/references/references.bib' are identified by the 'url' attribute.
- If the slug of an entry is used by another entry, whose url is different 
  from this entry, need to make its slug unique, such as adding a number to it.

#### 2.1.2 Replace References using Typst Syntax
The Typst syntax to use a reference is:
```text
@<reference-slug>
```

The original use of references in the Typst file is:
```text
([TypeSafe AI][1])
([TypeSafe AI][2])
```

These are replaced with:
```text
@introduction-typesafe-ai
@introducing-system-one-models-and-jev-typesafe-ai-blob
```