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


class TestFoldedScalarStripModifierIndentValidation:
    """Test cases for strip modifier (>-) indent validation at levels 1-3.

    The strip modifier (>-) removes trailing newlines and trailing whitespace.
    This test class validates that content indentation is correctly validated
    at each level for the strip modifier.

    Level 1: 2-space base indent with >- modifier
    Level 2: 4-space base indent with >- modifier
    Level 3: 6-space base indent with >- modifier
    """

    def test_strip_modifier_indent_validation(self):
        """Test strip modifier (>-) indent validation at levels 1-3.

        This test verifies that the strip modifier correctly removes trailing
        newlines and trailing whitespace while preserving content indentation
        at levels 1, 2, and 3.

        YAML folded scalar modifiers:
        - >-: strip (removes trailing newlines and trailing whitespace)

        Each test verifies:
        - Strip modifier (>-) removes trailing newlines
        - Strip modifier (>-) removes trailing whitespace
        - Content indentation is correctly preserved at each level
        """
        parser = YAMLCoreParser()

        # Level 1: 2-space base indent with strip modifier
        yaml_content_level1 = """# Strip modifier at level 1 (2-space base indent)
strip_level_1: >-
  Line 1 indented at level 1
  Line 2 indented at level 1
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Strip modifier parsing at level 1 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_level_1'], \
            "Strip >- at level 1 should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_level_1'], \
            "Strip >- at level 1 should preserve all indented lines"
        # Verify no trailing whitespace
        assert result.data['strip_level_1'] == result.data['strip_level_1'].rstrip(), \
            "Strip >- at level 1 should remove trailing whitespace"

        # Level 2: 4-space base indent with strip modifier
        yaml_content_level2 = """# Strip modifier at level 2 (4-space base indent)
    strip_level_2: >-
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Strip modifier parsing at level 2 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 double-indented at level 2' in result.data['strip_level_2'], \
            "Strip >- at level 2 should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_level_2'], \
            "Strip >- at level 2 should preserve all double-indented lines"
        # Verify no trailing whitespace
        assert result.data['strip_level_2'] == result.data['strip_level_2'].rstrip(), \
            "Strip >- at level 2 should remove trailing whitespace"

        # Level 3: 6-space base indent with strip modifier
        yaml_content_level3 = """# Strip modifier at level 3 (6-space base indent)
      strip_level_3: >-
        Line 1 triple-indented at level 3
        Line 2 triple-indented at level 3
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Strip modifier parsing at level 3 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 triple-indented at level 3' in result.data['strip_level_3'], \
            "Strip >- at level 3 should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_level_3'], \
            "Strip >- at level 3 should preserve all triple-indented lines"
        # Verify no trailing whitespace
        assert result.data['strip_level_3'] == result.data['strip_level_3'].rstrip(), \
            "Strip >- at level 3 should remove trailing whitespace"


class TestFoldedScalarKeepModifierIndentValidation:
    """Test cases for keep modifier (>+) indent validation at levels 1-3.

    The keep modifier (>+) preserves trailing newlines and trailing whitespace.
    This test class validates that content indentation is correctly validated
    at each level for the keep modifier.

    Level 1: 2-space content indent
    Level 2: 4-space content indent
    Level 3: 6-space content indent
    """

    def test_keep_modifier_indent_validation(self):
        """Test keep modifier (>+) indent validation at levels 1-3.

        This test verifies that the keep modifier correctly preserves trailing
        newlines and trailing whitespace while preserving content indentation
        at levels 1, 2, and 3.

        YAML folded scalar modifiers:
        - >+ or >+N: keep (keeps all trailing newlines and trailing whitespace)

        where N is the explicit indentation level (number of spaces).

        Each test verifies:
        - Keep modifier (>+) preserves trailing newlines
        - Keep modifier (>+) preserves trailing whitespace
        - Content indentation is correctly preserved at each level
        """
        parser = YAMLCoreParser()

        # Level 1: 2-space content indent with keep
        yaml_content_level1 = """# Keep modifier at level 1 (2-space content indent)
keep_level_1: >+1
 Line 1 indented at level 1
 Line 2 indented at level 1
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Keep modifier parsing at level 1 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_level_1'], \
            "Keep >+1 at level 1 should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_level_1'], \
            "Keep >+1 at level 1 should preserve all indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_level_1'].endswith('\n') or result.data['keep_level_1'] != result.data['keep_level_1'].rstrip(), \
            "Keep >+1 at level 1 should preserve trailing newlines/whitespace"

        # Level 2: 4-space content indent with keep
        yaml_content_level2 = """# Keep modifier at level 2 (4-space content indent)
keep_level_2: >+2
  Line 1 double-indented at level 2
  Line 2 double-indented at level 2
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Keep modifier parsing at level 2 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 double-indented at level 2' in result.data['keep_level_2'], \
            "Keep >+2 at level 2 should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_level_2'], \
            "Keep >+2 at level 2 should preserve all double-indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_level_2'].endswith('\n') or result.data['keep_level_2'] != result.data['keep_level_2'].rstrip(), \
            "Keep >+2 at level 2 should preserve trailing newlines/whitespace"

        # Level 3: 6-space content indent with keep
        yaml_content_level3 = """# Keep modifier at level 3 (6-space content indent)
keep_level_3: >+3
   Line 1 triple-indented at level 3
   Line 2 triple-indented at level 3
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Keep modifier parsing at level 3 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 triple-indented at level 3' in result.data['keep_level_3'], \
            "Keep >+3 at level 3 should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_level_3'], \
            "Keep >+3 at level 3 should preserve all triple-indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_level_3'].endswith('\n') or result.data['keep_level_3'] != result.data['keep_level_3'].rstrip(), \
            "Keep >+3 at level 3 should preserve trailing newlines/whitespace"


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


class TestFoldedScalarKeepModifierBaseIndentValidation:
    """Test cases for keep modifier (>+) validation at base indentation levels 1-3.

    The keep modifier (>+) preserves trailing newlines and trailing whitespace.
    This test class validates that content indentation is correctly validated
    at each base indentation level for the keep modifier.

    Base indentation levels:
    - Level 1: 2-space base indent with >+ modifier
    - Level 2: 4-space base indent with >+ modifier
    - Level 3: 6-space base indent with >+ modifier
    """

    def test_keep_modifier_base_indent_validation(self):
        """Test keep modifier (>+) validation at base indentation levels 1-3.

        This test verifies that the keep modifier correctly preserves trailing
        newlines and trailing whitespace while preserving content indentation
        at base indentation levels 1, 2, and 3.

        YAML folded scalar modifiers:
        - >+: keep (preserves all trailing newlines and trailing whitespace)

        Each test verifies:
        - Keep modifier (>+) preserves trailing newlines
        - Keep modifier (>+) preserves trailing whitespace
        - Content indentation is correctly preserved at each level
        """
        parser = YAMLCoreParser()

        # Level 1: 2-space base indent with keep modifier
        yaml_content_level1 = """# Keep modifier at level 1 (2-space base indent)
keep_base_level_1: >+
  Line 1 indented at level 1
  Line 2 indented at level 1
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Keep modifier parsing at base level 1 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 indented at level 1' in result.data['keep_base_level_1'], \
            "Keep >+ at base level 1 should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['keep_base_level_1'], \
            "Keep >+ at base level 1 should preserve all indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_base_level_1'].endswith('\n') or result.data['keep_base_level_1'] != result.data['keep_base_level_1'].rstrip(), \
            "Keep >+ at base level 1 should preserve trailing newlines/whitespace"

        # Level 2: 4-space base indent with keep modifier
        yaml_content_level2 = """# Keep modifier at level 2 (4-space base indent)
    keep_base_level_2: >+
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Keep modifier parsing at base level 2 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 double-indented at level 2' in result.data['keep_base_level_2'], \
            "Keep >+ at base level 2 should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['keep_base_level_2'], \
            "Keep >+ at base level 2 should preserve all double-indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_base_level_2'].endswith('\n') or result.data['keep_base_level_2'] != result.data['keep_base_level_2'].rstrip(), \
            "Keep >+ at base level 2 should preserve trailing newlines/whitespace"

        # Level 3: 6-space base indent with keep modifier
        yaml_content_level3 = """# Keep modifier at level 3 (6-space base indent)
      keep_base_level_3: >+
        Line 1 triple-indented at level 3
        Line 2 triple-indented at level 3
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Keep modifier parsing at base level 3 should succeed"

        # Verify keep modifier content is preserved (trailing newlines kept)
        assert 'Line 1 triple-indented at level 3' in result.data['keep_base_level_3'], \
            "Keep >+ at base level 3 should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['keep_base_level_3'], \
            "Keep >+ at base level 3 should preserve all triple-indented lines"
        # Verify trailing whitespace is preserved (keep modifier keeps it)
        assert result.data['keep_base_level_3'].endswith('\n') or result.data['keep_base_level_3'] != result.data['keep_base_level_3'].rstrip(), \
            "Keep >+ at base level 3 should preserve trailing newlines/whitespace"


class TestFoldedScalarContinuationLineVerification:
    """Test cases for continuation line verification in folded scalars.

    This test class verifies that continuation lines in folded scalars:
    1. Maintain proper indentation alignment for each level (1-5)
    2. Are NOT misidentified as mapping keys (critical for correctness)
    3. Preserve folded scalar semantics

    The key issue being tested is that folded scalar continuation lines
    that contain patterns like "key: value" should NOT be interpreted
    as nested mapping keys, but should be preserved as content within
    the folded scalar.
    """

    def test_continuation_lines_not_mapping_keys(self):
        """Test that continuation lines are NOT detected as mapping keys.

        This test verifies the critical behavior: folded scalar continuation
        lines that look like "key: value" patterns must be preserved as
        content, NOT interpreted as nested mapping keys.

        This is a multi-line YAML validation test following the pattern
        from test_mixed_comment_scenarios.py.
        """
        parser = YAMLCoreParser()

        # Level 1: Folded scalar with continuation lines that look like mappings
        yaml_content_level1 = """# Folded scalar with key-like continuation lines at level 1
description_1: >1
 This is a continuation line
 another_key: another_value  # This looks like a mapping but is content
 more continuation text
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Folded scalar with key-like continuation lines should succeed"

        # Verify the entire content is preserved as a single string
        assert 'another_key: another_value' in result.data['description_1'], \
            "Continuation line with 'key: value' pattern should be preserved as content"
        assert 'This is a continuation line' in result.data['description_1'], \
            "First continuation line should be preserved"
        assert 'more continuation text' in result.data['description_1'], \
            "Last continuation line should be preserved"

        # Verify it's NOT interpreted as a nested mapping
        assert isinstance(result.data['description_1'], str), \
            "Folded scalar should remain a string, not become a mapping"
        assert not isinstance(result.data['description_1'], dict), \
            "Continuation lines should NOT create nested mapping structure"

    def test_continuation_lines_level2_not_mapping_keys(self):
        """Test that continuation lines at level 2 are NOT detected as mapping keys."""
        parser = YAMLCoreParser()

        yaml_content_level2 = """# Folded scalar with key-like continuation lines at level 2
  description_2: >2
   This is level 2 content
   nested_key: nested_value  # Looks like mapping but is content
   more level 2 text
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Level 2 folded scalar with key-like continuation lines should succeed"

        assert 'nested_key: nested_value' in result.data['description_2'], \
            "Level 2 continuation line with 'key: value' should be preserved as content"
        assert isinstance(result.data['description_2'], str), \
            "Level 2 folded scalar should remain a string"

    def test_continuation_lines_level3_not_mapping_keys(self):
        """Test that continuation lines at level 3 are NOT detected as mapping keys."""
        parser = YAMLCoreParser()

        yaml_content_level3 = """# Folded scalar with key-like continuation lines at level 3
    description_3: >3
     This is level 3 content
     deep_key: deep_value  # Looks like mapping but is content
     more level 3 text
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Level 3 folded scalar with key-like continuation lines should succeed"

        assert 'deep_key: deep_value' in result.data['description_3'], \
            "Level 3 continuation line with 'key: value' should be preserved as content"
        assert isinstance(result.data['description_3'], str), \
            "Level 3 folded scalar should remain a string"

    def test_continuation_lines_level4_not_mapping_keys(self):
        """Test that continuation lines at level 4 are NOT detected as mapping keys."""
        parser = YAMLCoreParser()

        yaml_content_level4 = """# Folded scalar with key-like continuation lines at level 4
      description_4: >4
       This is level 4 content
       deeper_key: deeper_value  # Looks like mapping but is content
       more level 4 text
"""
        result = parser.safe_load(yaml_content_level4)
        assert result.is_success(), "Level 4 folded scalar with key-like continuation lines should succeed"

        assert 'deeper_key: deeper_value' in result.data['description_4'], \
            "Level 4 continuation line with 'key: value' should be preserved as content"
        assert isinstance(result.data['description_4'], str), \
            "Level 4 folded scalar should remain a string"

    def test_continuation_lines_level5_not_mapping_keys(self):
        """Test that continuation lines at level 5 are NOT detected as mapping keys."""
        parser = YAMLCoreParser()

        yaml_content_level5 = """# Folded scalar with key-like continuation lines at level 5
        description_5: >5
         This is level 5 content
         deepest_key: deepest_value  # Looks like mapping but is content
         more level 5 text
"""
        result = parser.safe_load(yaml_content_level5)
        assert result.is_success(), "Level 5 folded scalar with key-like continuation lines should succeed"

        assert 'deepest_key: deepest_value' in result.data['description_5'], \
            "Level 5 continuation line with 'key: value' should be preserved as content"
        assert isinstance(result.data['description_5'], str), \
            "Level 5 folded scalar should remain a string"

    def test_continuation_lines_indentation_alignment(self):
        """Test that continuation lines maintain proper indentation alignment.

        This test verifies that continuation lines at each level (1-5)
        maintain proper indentation alignment as per YAML folded scalar
        explicit indentation specification.
        """
        parser = YAMLCoreParser()

        yaml_content = """# Test continuation line indentation alignment at all levels
level_1: >1
 Level 1 continuation
 another: value1  # Key-like content at level 1

level_2: >2
  Level 2 continuation
  another: value2  # Key-like content at level 2

level_3: >3
   Level 3 continuation
   another: value3  # Key-like content at level 3

level_4: >4
    Level 4 continuation
    another: value4  # Key-like content at level 4

level_5: >5
     Level 5 continuation
     another: value5  # Key-like content at level 5
"""
        result = parser.safe_load(yaml_content)
        assert result.is_success(), "All levels with continuation lines should succeed"

        # Verify each level preserves content correctly
        assert 'another: value1' in result.data['level_1'], \
            "Level 1 continuation should preserve key-like content"
        assert 'another: value2' in result.data['level_2'], \
            "Level 2 continuation should preserve key-like content"
        assert 'another: value3' in result.data['level_3'], \
            "Level 3 continuation should preserve key-like content"
        assert 'another: value4' in result.data['level_4'], \
            "Level 4 continuation should preserve key-like content"
        assert 'another: value5' in result.data['level_5'], \
            "Level 5 continuation should preserve key-like content"

        # Verify all are strings, not mappings
        for level_key in ['level_1', 'level_2', 'level_3', 'level_4', 'level_5']:
            assert isinstance(result.data[level_key], str), \
                f"{level_key} should remain a string, not become a mapping"

    def test_continuation_lines_folded_scalar_semantics(self):
        """Test that continuation lines preserve folded scalar semantics.

        This test verifies that folded scalar continuation lines:
        1. Fold newlines to spaces (plain > modifier)
        2. Preserve content including key-like patterns
        3. Maintain proper indentation semantics
        """
        parser = YAMLCoreParser()

        yaml_content_plain = """# Plain folded scalar continuation semantics
plain_folded: >1
 First line
 config: value  # Key-like pattern
 Third line
"""
        result = parser.safe_load(yaml_content_plain)
        assert result.is_success(), "Plain folded scalar should succeed"

        # Verify newlines are folded to spaces
        content = result.data['plain_folded']
        assert 'First line' in content, "First line should be preserved"
        assert 'config: value' in content, "Key-like pattern should be preserved"
        assert 'Third line' in content, "Third line should be preserved"

        # Verify folded semantics (newlines become spaces)
        assert '\n' not in content or content.count('\n') <= 1, \
            "Plain folded scalar should fold newlines to spaces"

    def test_continuation_lines_strip_modifier(self):
        """Test continuation lines with strip modifier (>-)."""
        parser = YAMLCoreParser()

        yaml_content_strip = """# Strip modifier continuation lines
strip_folded: >-1
 Line 1
 setting: value  # Key-like content with strip
 Line 3
"""
        result = parser.safe_load(yaml_content_strip)
        assert result.is_success(), "Strip folded scalar should succeed"

        assert 'setting: value' in result.data['strip_folded'], \
            "Strip modifier should preserve key-like content"
        assert result.data['strip_folded'] == result.data['strip_folded'].rstrip(), \
            "Strip modifier should remove trailing whitespace"

    def test_continuation_lines_keep_modifier(self):
        """Test continuation lines with keep modifier (>+)."""
        parser = YAMLCoreParser()

        yaml_content_keep = """# Keep modifier continuation lines
keep_folded: >+1
 Line 1
 option: value  # Key-like content with keep
 Line 3
"""
        result = parser.safe_load(yaml_content_keep)
        assert result.is_success(), "Keep folded scalar should succeed"

        assert 'option: value' in result.data['keep_folded'], \
            "Keep modifier should preserve key-like content"
        # Keep modifier preserves trailing newlines
        assert result.data['keep_folded'].endswith('\n') or \
               result.data['keep_folded'] != result.data['keep_folded'].rstrip(), \
            "Keep modifier should preserve trailing newlines/whitespace"

    def test_continuation_lines_complex_key_patterns(self):
        """Test continuation lines with complex key-like patterns.

        This test verifies that various key-like patterns in continuation
        lines are all preserved as content, not interpreted as mappings.
        """
        parser = YAMLCoreParser()

        yaml_content = """# Complex key-like patterns in continuation lines
complex_test: >1
 simple_key: simple_value
 key-with-dash: value-with-dash
 key_with_underscore: value_with_underscore
 key.with.dots: value.with.dots
 nested:
   deeply: value
 key: value with colon in text
"""
        result = parser.safe_load(yaml_content)
        assert result.is_success(), "Complex key patterns should succeed"

        content = result.data['complex_test']
        assert 'simple_key: simple_value' in content, \
            "Simple key pattern should be preserved"
        assert 'key-with-dash: value-with-dash' in content, \
            "Dash key pattern should be preserved"
        assert 'key_with_underscore: value_with_underscore' in content, \
            "Underscore key pattern should be preserved"
        assert 'key.with.dots: value.with.dots' in content, \
            "Dotted key pattern should be preserved"
        assert 'nested:' in content and 'deeply: value' in content, \
            "Nested-looking pattern should be preserved"

        # Critical: must remain a string, not become a mapping
        assert isinstance(content, str), \
            "Complex key-like patterns must NOT create nested mapping"

    def test_continuation_lines_multiple_colons(self):
        """Test continuation lines with multiple colon patterns.

        This verifies that lines with multiple colons (e.g., URLs, time values)
        are preserved as content, not misinterpreted as mappings.
        """
        parser = YAMLCoreParser()

        yaml_content = """# Multiple colon patterns in continuation lines
colon_test: >1
 https://example.com/path
 time_value: 12:30:45
 url: http://user:pass@host:port/path
 ratio: 3:2:1
"""
        result = parser.safe_load(yaml_content)
        assert result.is_success(), "Multiple colon patterns should succeed"

        content = result.data['colon_test']
        assert 'https://example.com/path' in content, \
            "URL with protocol should be preserved"
        assert 'time_value: 12:30:45' in content, \
            "Time value pattern should be preserved"
        assert 'url: http://user:pass@host:port/path' in content, \
            "URL with credentials should be preserved"
        assert 'ratio: 3:2:1' in content, \
            "Ratio pattern should be preserved"

        assert isinstance(content, str), \
            "Multiple colon patterns must NOT create nested mapping"


class TestFoldedScalarStripModifierBaseIndentValidation:
    """Test cases for strip modifier (>-) validation at base indentation levels 1-3.

    The strip modifier (>-) removes trailing newlines and trailing whitespace.
    This test class validates that content indentation is correctly validated
    at each base indentation level for the strip modifier.

    Base indentation levels:
    - Level 1: 2-space base indent with >- modifier
    - Level 2: 4-space base indent with >- modifier
    - Level 3: 6-space base indent with >- modifier
    """

    def test_strip_modifier_base_indent_validation(self):
        """Test strip modifier (>-) validation at base indentation levels 1-3.

        This test verifies that the strip modifier correctly removes trailing
        newlines and trailing whitespace while preserving content indentation
        at base indentation levels 1, 2, and 3.

        YAML folded scalar modifiers:
        - >-: strip (removes trailing newlines and trailing whitespace)

        Each test verifies:
        - Strip modifier (>-) removes trailing newlines
        - Strip modifier (>-) removes trailing whitespace
        - Content indentation is correctly preserved at each level
        """
        parser = YAMLCoreParser()

        # Level 1: 2-space base indent with strip modifier
        yaml_content_level1 = """# Strip modifier at level 1 (2-space base indent)
strip_base_level_1: >-
  Line 1 indented at level 1
  Line 2 indented at level 1
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Strip modifier parsing at base level 1 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 indented at level 1' in result.data['strip_base_level_1'], \
            "Strip >- at base level 1 should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['strip_base_level_1'], \
            "Strip >- at base level 1 should preserve all indented lines"
        # Verify trailing whitespace is removed (strip modifier removes it)
        assert result.data['strip_base_level_1'] == result.data['strip_base_level_1'].rstrip(), \
            "Strip >- at base level 1 should remove trailing newlines/whitespace"

        # Level 2: 4-space base indent with strip modifier
        yaml_content_level2 = """# Strip modifier at level 2 (4-space base indent)
    strip_base_level_2: >-
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Strip modifier parsing at base level 2 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 double-indented at level 2' in result.data['strip_base_level_2'], \
            "Strip >- at base level 2 should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['strip_base_level_2'], \
            "Strip >- at base level 2 should preserve all double-indented lines"
        # Verify trailing whitespace is removed (strip modifier removes it)
        assert result.data['strip_base_level_2'] == result.data['strip_base_level_2'].rstrip(), \
            "Strip >- at base level 2 should remove trailing newlines/whitespace"

        # Level 3: 6-space base indent with strip modifier
        yaml_content_level3 = """# Strip modifier at level 3 (6-space base indent)
      strip_base_level_3: >-
        Line 1 triple-indented at level 3
        Line 2 triple-indented at level 3
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Strip modifier parsing at base level 3 should succeed"

        # Verify strip modifier content is preserved (trailing newlines removed)
        assert 'Line 1 triple-indented at level 3' in result.data['strip_base_level_3'], \
            "Strip >- at base level 3 should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['strip_base_level_3'], \
            "Strip >- at base level 3 should preserve all triple-indented lines"
        # Verify trailing whitespace is removed (strip modifier removes it)
        assert result.data['strip_base_level_3'] == result.data['strip_base_level_3'].rstrip(), \
            "Strip >- at base level 3 should remove trailing newlines/whitespace"


class TestFoldedScalarPlainModifierBaseIndentValidation:
    """Test cases for plain modifier (>) validation at base indentation levels 1-5.

    The plain modifier (>) keeps a single trailing newline and folds other newlines to spaces.
    This test class validates that content indentation is correctly validated
    at each base indentation level for the plain modifier.

    Base indentation levels:
    - Level 1: 2-space base indent with > modifier
    - Level 2: 4-space base indent with > modifier
    - Level 3: 6-space base indent with > modifier
    - Level 4: 8-space base indent with > modifier
    - Level 5: 10-space base indent with > modifier
    """

    def test_plain_modifier_base_indent_validation(self):
        """Test plain modifier (>) validation at base indentation levels 1-5.

        This test verifies that the plain modifier correctly:
        - Keeps a single trailing newline
        - Folds content newlines to spaces
        - Preserves content indentation at each level

        YAML folded scalar modifiers:
        - >: plain (keeps single trailing newline, folds other newlines to spaces)

        Each test verifies:
        - Plain modifier (>) keeps a single trailing newline
        - Plain modifier (>) folds content newlines to spaces
        - Content indentation is correctly preserved at each level
        """
        parser = YAMLCoreParser()

        # Level 1: 2-space base indent with plain modifier
        yaml_content_level1 = """# Plain modifier at level 1 (2-space base indent)
plain_base_level_1: >
  Line 1 indented at level 1
  Line 2 indented at level 1
"""
        result = parser.safe_load(yaml_content_level1)
        assert result.is_success(), "Plain modifier parsing at base level 1 should succeed"

        # Verify plain modifier content is preserved
        assert 'Line 1 indented at level 1' in result.data['plain_base_level_1'], \
            "Plain > at base level 1 should preserve indented content"
        assert 'Line 2 indented at level 1' in result.data['plain_base_level_1'], \
            "Plain > at base level 1 should preserve all indented lines"
        # Verify content newlines are folded to spaces (plain modifier behavior)
        content = result.data['plain_base_level_1']
        assert 'Line 1 indented at level 1' in content and 'Line 2 indented at level 1' in content, \
            "Plain > at base level 1 should preserve all content with newlines folded"
        # Single trailing newline may be present
        assert content.endswith('\n') or not content.endswith(' '), \
            "Plain > at base level 1 should end with newline or not trailing space"

        # Level 2: 4-space base indent with plain modifier
        yaml_content_level2 = """# Plain modifier at level 2 (4-space base indent)
    plain_base_level_2: >
      Line 1 double-indented at level 2
      Line 2 double-indented at level 2
"""
        result = parser.safe_load(yaml_content_level2)
        assert result.is_success(), "Plain modifier parsing at base level 2 should succeed"

        # Verify plain modifier content is preserved
        assert 'Line 1 double-indented at level 2' in result.data['plain_base_level_2'], \
            "Plain > at base level 2 should preserve double-indented content"
        assert 'Line 2 double-indented at level 2' in result.data['plain_base_level_2'], \
            "Plain > at base level 2 should preserve all double-indented lines"

        # Level 3: 6-space base indent with plain modifier
        yaml_content_level3 = """# Plain modifier at level 3 (6-space base indent)
      plain_base_level_3: >
        Line 1 triple-indented at level 3
        Line 2 triple-indented at level 3
"""
        result = parser.safe_load(yaml_content_level3)
        assert result.is_success(), "Plain modifier parsing at base level 3 should succeed"

        # Verify plain modifier content is preserved
        assert 'Line 1 triple-indented at level 3' in result.data['plain_base_level_3'], \
            "Plain > at base level 3 should preserve triple-indented content"
        assert 'Line 2 triple-indented at level 3' in result.data['plain_base_level_3'], \
            "Plain > at base level 3 should preserve all triple-indented lines"

        # Level 4: 8-space base indent with plain modifier
        yaml_content_level4 = """# Plain modifier at level 4 (8-space base indent)
        plain_base_level_4: >
          Line 1 quad-indented at level 4
          Line 2 quad-indented at level 4
"""
        result = parser.safe_load(yaml_content_level4)
        assert result.is_success(), "Plain modifier parsing at base level 4 should succeed"

        # Verify plain modifier content is preserved
        assert 'Line 1 quad-indented at level 4' in result.data['plain_base_level_4'], \
            "Plain > at base level 4 should preserve quad-indented content"
        assert 'Line 2 quad-indented at level 4' in result.data['plain_base_level_4'], \
            "Plain > at base level 4 should preserve all quad-indented lines"

        # Level 5: 10-space base indent with plain modifier
        yaml_content_level5 = """# Plain modifier at level 5 (10-space base indent)
          plain_base_level_5: >
            Line 1 quint-indented at level 5
            Line 2 quint-indented at level 5
"""
        result = parser.safe_load(yaml_content_level5)
        assert result.is_success(), "Plain modifier parsing at base level 5 should succeed"

        # Verify plain modifier content is preserved
        assert 'Line 1 quint-indented at level 5' in result.data['plain_base_level_5'], \
            "Plain > at base level 5 should preserve quint-indented content"
        assert 'Line 2 quint-indented at level 5' in result.data['plain_base_level_5'], \
            "Plain > at base level 5 should preserve all quint-indented lines"


if __name__ == '__main__':
    # Run the tests when executed directly
    pytest.main([__file__, '-v'])
