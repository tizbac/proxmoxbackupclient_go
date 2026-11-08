#!/usr/bin/env python3
"""Joins hard-wrapped Markdown paragraphs back into single lines.

A GitHub release body keeps every line break, so text wrapped at ~70
columns in its sources (the workflow's template, CHANGELOG.md) shows as a
narrow column there. This joins the lines of each paragraph and list item;
headings, tables, code blocks, HTML, quotes and blank lines are left as they
are, and a line ending in two spaces (a deliberate break) is kept.

Usage: unwrap-markdown.py FILE        (rewrites FILE)
       unwrap-markdown.py < IN > OUT
"""
import re
import sys

BLOCK_START = re.compile(r'^\s*(#{1,6}\s|\||```|~~~|<|>|---+\s*$|\*\*\*+\s*$|[-*+]\s|\d+[.)]\s)')
LIST_ITEM = re.compile(r'^\s*([-*+]|\d+[.)])\s')


def unwrap(text):
    out = []
    fence = False
    for line in text.split('\n'):
        s = line.rstrip('\r')
        if s.lstrip().startswith(('```', '~~~')):
            fence = not fence
            out.append(s)
            continue
        if fence:
            out.append(s)
            continue
        prev = out[-1] if out else ''
        joinable_prev = (
            prev.strip() != ''
            and not prev.lstrip().startswith(('#', '|', '<', '```', '~~~'))
            and not re.match(r'^\s*(---+|\*\*\*+)\s*$', prev)
            and not prev.endswith('  ')
        )
        # A continuation: a non-blank line that does not start a new block.
        # An indented line under a list item continues that item.
        continuation = s.strip() != '' and not BLOCK_START.match(s)
        # A quote continues a quote.
        if prev.lstrip().startswith('>') and s.lstrip().startswith('>') and s.lstrip() not in ('>',):
            continuation = prev.strip() not in ('>',)
            if continuation:
                out[-1] = prev.rstrip() + ' ' + s.lstrip()[1:].lstrip()
                continue
        if joinable_prev and continuation and not prev.lstrip().startswith('>'):
            out[-1] = prev.rstrip() + ' ' + s.strip()
        else:
            out.append(s)
    return '\n'.join(out)


def main():
    if len(sys.argv) > 1:
        path = sys.argv[1]
        with open(path, encoding='utf-8') as f:
            text = f.read()
        with open(path, 'w', encoding='utf-8', newline='\n') as f:
            f.write(unwrap(text))
    else:
        sys.stdout.write(unwrap(sys.stdin.read()))


if __name__ == '__main__':
    main()
