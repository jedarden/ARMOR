"""
Tests for YAML folded scalar explicit indentation.

This module tests folded scalar behavior with explicit indentation levels.
YAML folded scalars use the > modifier with optional indentation specification.

YAML folded scalar modifiers:
- > or >N  : plain (keeps single trailing newline)
- >- or >-N: strip (removes trailing newlines)
- >+ or >+N: keep (keeps all trailing newlines)

where N is the explicit indentation level (number of spaces).

This module tests explicit indentation at various base indentation levels:
- 0-space base (no indentation) - content indented at 2, 4, 6, 8, 10 spaces
- 2-space base (Level 1)
- 4-space base (Level 2)
- 6-space base (Level 3)
- 8-space base (Level 4)
- 10-space base (Level 5)
"""

import pytest
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.parser import YAMLCoreParser


class TestFoldedScalarExplicitIndent0Space:
    """Test cases for folded scalars with explicit indent at 0-space base indentation.

    At 0-space base indentation, the key is not indented and content
    lines are indented with N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_0space(self):
        """Test folded scalars with explicit indent at 0-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 0-space base indentation (no indentation).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 0-space base indentation, the key is not indented and content
        lines are indented with N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 0-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 0-space level
plain_indent_1: >1
 Line 1 indented at level 1
 Line 2 indented at level 1

plain_indent_2: >2
  Line 1 double-indented at level 2
  Line 2 double-indented at level 2

plain_indent_3: >3
   Line 1 triple-indented at level 3
   Line 2 triple-indented at level 3

plain_indent_4: >4
    Line 1 quad-indented at level 4
    Line 2 quad-indented at level 4

plain_indent_5: >5
     Line 1 quint-indented at level 5
     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 0-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 0-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 0-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 0-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 0-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 0-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 0-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 0-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 0-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 0-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 0-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 0-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 0-space level
strip_indent_1: >-1
 Line 1 indented at level 1
 Line 2 indented at level 1

strip_indent_2: >-2
  Line 1 double-indented at level 2
  Line 2 double-indented at level 2

strip_indent_3: >-3
   Line 1 triple-indented at level 3
   Line 2 triple-indented at level 3

strip_indent_4: >-4
    Line 1 quad-indented at level 4
    Line 2 quad-indented at level 4

strip_indent_5: >-5
     Line 1 quint-indented at level 5
     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 0-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 0-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 0-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 0-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 0-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 0-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 0-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 0-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 0-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 0-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 0-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 0-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 0-space level
keep_indent_1: >+1
 Line 1 indented at level 1
 Line 2 indented at level 1

keep_indent_2: >+2
  Line 1 double-indented at level 2
  Line 2 double-indented at level 2

keep_indent_3: >+3
   Line 1 triple-indented at level 3
   Line 2 triple-indented at level 3

keep_indent_4: >+4
    Line 1 quad-indented at level 4
    Line 2 quad-indented at level 4

keep_indent_5: >+5
     Line 1 quint-indented at level 5
     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 0-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 0-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 0-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 0-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 0-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 0-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 0-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 0-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 0-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 0-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 0-space base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndent2Space:
    """Test cases for folded scalars with explicit indent at 2-space base indentation.

    At 2-space base indentation, the key is indented with 2 spaces and content
    lines are indented with 2 + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_2space(self):
        """Test folded scalars with explicit indent at 2-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 2-space base indentation (Level 1).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 2-space base indentation, the key is indented with 2 spaces and content
        lines are indented with 2 + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 2-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 2-space level
  plain_indent_1: >1
   Line 1 indented at level 1
   Line 2 indented at level 1

  plain_indent_2: >2
    Line 1 double-indented at level 2
    Line 2 double-indented at level 2

  plain_indent_3: >3
     Line 1 triple-indented at level 3
     Line 2 triple-indented at level 3

  plain_indent_4: >4
      Line 1 quad-indented at level 4
      Line 2 quad-indented at level 4

  plain_indent_5: >5
       Line 1 quint-indented at level 5
       Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 2-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 2-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 2-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 2-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 2-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 2-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 2-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 2-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 2-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 2-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 2-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 2-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 2-space level
  strip_indent_1: >-1
   Line 1 indented at level 1
   Line 2 indented at level 1

  strip_indent_2: >-2
    Line 1 double-indented at level 2
    Line 2 double-indented at level 2

  strip_indent_3: >-3
     Line 1 triple-indented at level 3
     Line 2 triple-indented at level 3

  strip_indent_4: >-4
      Line 1 quad-indented at level 4
      Line 2 quad-indented at level 4

  strip_indent_5: >-5
       Line 1 quint-indented at level 5
       Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 2-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 2-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 2-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 2-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 2-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 2-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 2-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 2-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 2-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 2-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 2-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 2-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 2-space level
  keep_indent_1: >+1
   Line 1 indented at level 1
   Line 2 indented at level 1

  keep_indent_2: >+2
    Line 1 double-indented at level 2
    Line 2 double-indented at level 2

  keep_indent_3: >+3
     Line 1 triple-indented at level 3
     Line 2 triple-indented at level 3

  keep_indent_4: >+4
      Line 1 quad-indented at level 4
      Line 2 quad-indented at level 4

  keep_indent_5: >+5
       Line 1 quint-indented at level 5
       Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 2-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 2-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 2-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 2-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 2-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 2-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 2-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 2-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 2-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 2-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 2-space base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndent4Space:
    """Test cases for folded scalars with explicit indent at 4-space base indentation.

    At 4-space base indentation, the key is indented with 4 spaces and content
    lines are indented with 4 + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_4space(self):
        """Test folded scalars with explicit indent at 4-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 4-space base indentation (Level 2).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 4-space base indentation, the key is indented with 4 spaces and content
        lines are indented with 4 + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 4-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 4-space level
    plain_indent_1: >1
     Line 1 indented at level 1
     Line 2 indented at level 1

    plain_indent_2: >2
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2

    plain_indent_3: >3
       Line 1 triple-indented at level 3
       Line 2 triple-indented at level 3

    plain_indent_4: >4
        Line 1 quad-indented at level 4
        Line 2 quad-indented at level 4

    plain_indent_5: >5
         Line 1 quint-indented at level 5
         Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 4-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 4-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 4-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 4-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 4-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 4-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 4-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 4-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 4-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 4-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 4-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 4-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 4-space level
    strip_indent_1: >-1
     Line 1 indented at level 1
     Line 2 indented at level 1

    strip_indent_2: >-2
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2

    strip_indent_3: >-3
       Line 1 triple-indented at level 3
       Line 2 triple-indented at level 3

    strip_indent_4: >-4
        Line 1 quad-indented at level 4
        Line 2 quad-indented at level 4

    strip_indent_5: >-5
         Line 1 quint-indented at level 5
         Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 4-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 4-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 4-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 4-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 4-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 4-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 4-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 4-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 4-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 4-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 4-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 4-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 4-space level
    keep_indent_1: >+1
     Line 1 indented at level 1
     Line 2 indented at level 1

    keep_indent_2: >+2
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2

    keep_indent_3: >+3
       Line 1 triple-indented at level 3
       Line 2 triple-indented at level 3

    keep_indent_4: >+4
        Line 1 quad-indented at level 4
        Line 2 quad-indented at level 4

    keep_indent_5: >+5
         Line 1 quint-indented at level 5
         Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 4-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 4-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 4-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 4-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 4-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 4-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 4-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 4-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 4-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 4-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 4-space base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndentTab:
    """Test cases for folded scalars with explicit indent at tab base indentation.

    At tab base indentation, the key is indented with 1 tab and content
    lines are indented with 1 tab + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_tab(self):
        """Test folded scalars with explicit indent at tab base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at tab base indentation.

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At tab base indentation, the key is indented with 1 tab and content
        lines are indented with 1 tab + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using tab base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at tab level
\tplain_indent_1: >1
\t Line 1 indented at level 1
\t Line 2 indented at level 1

\tplain_indent_2: >2
\t  Line 1 double-indented at level 2
\t  Line 2 double-indented at level 2

\tplain_indent_3: >3
\t   Line 1 triple-indented at level 3
\t   Line 2 triple-indented at level 3

\tplain_indent_4: >4
\t    Line 1 quad-indented at level 4
\t    Line 2 quad-indented at level 4

\tplain_indent_5: >5
\t     Line 1 quint-indented at level 5
\t     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with tab base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with tab base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with tab base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with tab base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with tab base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with tab base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with tab base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with tab base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with tab base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with tab base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with tab base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using tab base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at tab level
\tstrip_indent_1: >-1
\t Line 1 indented at level 1
\t Line 2 indented at level 1

\tstrip_indent_2: >-2
\t  Line 1 double-indented at level 2
\t  Line 2 double-indented at level 2

\tstrip_indent_3: >-3
\t   Line 1 triple-indented at level 3
\t   Line 2 triple-indented at level 3

\tstrip_indent_4: >-4
\t    Line 1 quad-indented at level 4
\t    Line 2 quad-indented at level 4

\tstrip_indent_5: >-5
\t     Line 1 quint-indented at level 5
\t     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with tab base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with tab base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with tab base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with tab base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with tab base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with tab base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with tab base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with tab base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with tab base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with tab base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with tab base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using tab base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at tab level
\tkeep_indent_1: >+1
\t Line 1 indented at level 1
\t Line 2 indented at level 1

\tkeep_indent_2: >+2
\t  Line 1 double-indented at level 2
\t  Line 2 double-indented at level 2

\tkeep_indent_3: >+3
\t   Line 1 triple-indented at level 3
\t   Line 2 triple-indented at level 3

\tkeep_indent_4: >+4
\t    Line 1 quad-indented at level 4
\t    Line 2 quad-indented at level 4

\tkeep_indent_5: >+5
\t     Line 1 quint-indented at level 5
\t     Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with tab base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with tab base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with tab base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with tab base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with tab base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with tab base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with tab base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with tab base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with tab base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with tab base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with tab base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndent6Space:
    """Test cases for folded scalars with explicit indent at 6-space base indentation.

    At 6-space base indentation, the key is indented with 6 spaces and content
    lines are indented with 6 + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_6space(self):
        """Test folded scalars with explicit indent at 6-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 6-space base indentation (Level 3).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 6-space base indentation, the key is indented with 6 spaces and content
        lines are indented with 6 + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 6-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 6-space level
      plain_indent_1: >1
       Line 1 indented at level 1
       Line 2 indented at level 1

      plain_indent_2: >2
        Line 1 double-indented at level 2
        Line 2 double-indented at level 2

      plain_indent_3: >3
         Line 1 triple-indented at level 3
         Line 2 triple-indented at level 3

      plain_indent_4: >4
          Line 1 quad-indented at level 4
          Line 2 quad-indented at level 4

      plain_indent_5: >5
           Line 1 quint-indented at level 5
           Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 6-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 6-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 6-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 6-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 6-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 6-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 6-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 6-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 6-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 6-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 6-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 6-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 6-space level
      strip_indent_1: >-1
       Line 1 indented at level 1
       Line 2 indented at level 1

      strip_indent_2: >-2
        Line 1 double-indented at level 2
        Line 2 double-indented at level 2

      strip_indent_3: >-3
         Line 1 triple-indented at level 3
         Line 2 triple-indented at level 3

      strip_indent_4: >-4
          Line 1 quad-indented at level 4
          Line 2 quad-indented at level 4

      strip_indent_5: >-5
           Line 1 quint-indented at level 5
           Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 6-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 6-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 6-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 6-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 6-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 6-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 6-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 6-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 6-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 6-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 6-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 6-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 6-space level
      keep_indent_1: >+1
       Line 1 indented at level 1
       Line 2 indented at level 1

      keep_indent_2: >+2
        Line 1 double-indented at level 2
        Line 2 double-indented at level 2

      keep_indent_3: >+3
         Line 1 triple-indented at level 3
         Line 2 triple-indented at level 3

      keep_indent_4: >+4
          Line 1 quad-indented at level 4
          Line 2 quad-indented at level 4

      keep_indent_5: >+5
           Line 1 quint-indented at level 5
           Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 6-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 6-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 6-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 6-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 6-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 6-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 6-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 6-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 6-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 6-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 6-space base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndent8Space:
    """Test cases for folded scalars with explicit indent at 8-space base indentation.

    At 8-space base indentation, the key is indented with 8 spaces and content
    lines are indented with 8 + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_8space(self):
        """Test folded scalars with explicit indent at 8-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 8-space base indentation (Level 4).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 8-space base indentation, the key is indented with 8 spaces and content
        lines are indented with 8 + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 8-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 8-space level
        plain_indent_1: >1
         Line 1 indented at level 1
         Line 2 indented at level 1

        plain_indent_2: >2
          Line 1 double-indented at level 2
          Line 2 double-indented at level 2

        plain_indent_3: >3
           Line 1 triple-indented at level 3
           Line 2 triple-indented at level 3

        plain_indent_4: >4
            Line 1 quad-indented at level 4
            Line 2 quad-indented at level 4

        plain_indent_5: >5
             Line 1 quint-indented at level 5
             Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 8-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 8-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 8-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 8-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 8-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 8-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 8-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 8-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 8-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 8-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 8-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 8-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 8-space level
        strip_indent_1: >-1
         Line 1 indented at level 1
         Line 2 indented at level 1

        strip_indent_2: >-2
          Line 1 double-indented at level 2
          Line 2 double-indented at level 2

        strip_indent_3: >-3
           Line 1 triple-indented at level 3
           Line 2 triple-indented at level 3

        strip_indent_4: >-4
            Line 1 quad-indented at level 4
            Line 2 quad-indented at level 4

        strip_indent_5: >-5
             Line 1 quint-indented at level 5
             Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 8-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 8-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 8-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 8-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 8-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 8-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 8-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 8-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 8-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 8-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 8-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 8-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 8-space level
        keep_indent_1: >+1
         Line 1 indented at level 1
         Line 2 indented at level 1

        keep_indent_2: >+2
          Line 1 double-indented at level 2
          Line 2 double-indented at level 2

        keep_indent_3: >+3
           Line 1 triple-indented at level 3
           Line 2 triple-indented at level 3

        keep_indent_4: >+4
            Line 1 quad-indented at level 4
            Line 2 quad-indented at level 4

        keep_indent_5: >+5
             Line 1 quint-indented at level 5
             Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 8-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 8-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 8-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 8-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 8-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 8-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 8-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 8-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 8-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 8-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 8-space base should preserve all quint-indented lines"


class TestFoldedScalarExplicitIndent10Space:
    """Test cases for folded scalars with explicit indent at 10-space base indentation.

    At 10-space base indentation, the key is indented with 10 spaces and content
    lines are indented with 10 + N spaces, where N is the explicit indent level.
    """

    def test_folded_scalar_explicit_indent_10space(self):
        """Test folded scalars with explicit indent at 10-space base indentation.

        This test verifies that folded scalars with explicit indentation work correctly
        at 10-space base indentation (Level 5).

        YAML folded scalar modifiers:
        - > or >N  : plain (keeps single trailing newline)
        - >- or >-N: strip (removes trailing newlines)
        - >+ or >+N: keep (keeps all trailing newlines)

        where N is the explicit indentation level (number of spaces).

        At 10-space base indentation, the key is indented with 10 spaces and content
        lines are indented with 10 + N spaces.
        """
        parser = YAMLCoreParser()

        # Test modifier > (plain) with indent levels 1-5, using 10-space base indentation
        yaml_content_plain = """# Plain folded scalar with explicit indent at 10-space level
          plain_indent_1: >1
           Line 1 indented at level 1
           Line 2 indented at level 1

          plain_indent_2: >2
            Line 1 double-indented at level 2
            Line 2 double-indented at level 2

          plain_indent_3: >3
             Line 1 triple-indented at level 3
             Line 2 triple-indented at level 3

          plain_indent_4: >4
              Line 1 quad-indented at level 4
              Line 2 quad-indented at level 4

          plain_indent_5: >5
               Line 1 quint-indented at level 5
               Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar parsing with 10-space base should succeed"

        # Verify plain modifier content is preserved (newlines folded to spaces)
        assert 'Line 1 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 10-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_indent_1'], \
            "Plain >1 with 10-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 10-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_indent_2'], \
            "Plain >2 with 10-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 10-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_indent_3'], \
            "Plain >3 with 10-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 10-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_indent_4'], \
            "Plain >4 with 10-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 10-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_indent_5'], \
            "Plain >5 with 10-space base should preserve all quint-indented lines"

        # Test modifier >- (strip) with indent levels 1-5, using 10-space base indentation
        yaml_content_strip = """# Strip folded scalar with explicit indent at 10-space level
          strip_indent_1: >-1
           Line 1 indented at level 1
           Line 2 indented at level 1

          strip_indent_2: >-2
            Line 1 double-indented at level 2
            Line 2 double-indented at level 2

          strip_indent_3: >-3
             Line 1 triple-indented at level 3
             Line 2 triple-indented at level 3

          strip_indent_4: >-4
              Line 1 quad-indented at level 4
              Line 2 quad-indented at level 4

          strip_indent_5: >-5
               Line 1 quint-indented at level 5
               Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar parsing with 10-space base should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 10-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_indent_1'], \
            "Strip >-1 with 10-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 10-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_indent_2'], \
            "Strip >-2 with 10-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 10-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_indent_3'], \
            "Strip >-3 with 10-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 10-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['strip_indent_4'], \
            "Strip >-4 with 10-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 10-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['strip_indent_5'], \
            "Strip >-5 with 10-space base should preserve all quint-indented lines"

        # Test modifier >+ (keep) with indent levels 1-5, using 10-space base indentation
        yaml_content_keep = """# Keep folded scalar with explicit indent at 10-space level
          keep_indent_1: >+1
           Line 1 indented at level 1
           Line 2 indented at level 1

          keep_indent_2: >+2
            Line 1 double-indented at level 2
            Line 2 double-indented at level 2

          keep_indent_3: >+3
             Line 1 triple-indented at level 3
             Line 2 triple-indented at level 3

          keep_indent_4: >+4
              Line 1 quad-indented at level 4
              Line 2 quad-indented at level 4

          keep_indent_5: >+5
               Line 1 quint-indented at level 5
               Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar parsing with 10-space base should succeed"

        # Verify keep modifier content is preserved (all trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 10-space base should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_indent_1'], \
            "Keep >+1 with 10-space base should preserve all indented lines"

        assert 'Line 1 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 10-space base should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_indent_2'], \
            "Keep >+2 with 10-space base should preserve all double-indented lines"

        assert 'Line 1 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 10-space base should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_indent_3'], \
            "Keep >+3 with 10-space base should preserve all triple-indented lines"

        assert 'Line 1 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 10-space base should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['keep_indent_4'], \
            "Keep >+4 with 10-space base should preserve all quad-indented lines"

        assert 'Line 1 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 10-space base should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['keep_indent_5'], \
            "Keep >+5 with 10-space base should preserve all quint-indented lines"


if __name__ == '__main__':
    # Run the tests when executed directly
    pytest.main([__file__, '-v'])
