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
- 0-space base (no indentation)
- 2-space base (Level 1)
- 4-space base (Level 2)
"""

import pytest
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.parser import YAMLCoreParser


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


if __name__ == '__main__':
    # Run the tests when executed directly
    pytest.main([__file__, '-v'])
