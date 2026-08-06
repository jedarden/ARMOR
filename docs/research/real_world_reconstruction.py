#!/usr/bin/env python3
"""
Reconstruct the real-world failure from queue-db and forgejo-postgres.
"""

import tarfile
import io

ARMOR_BLOCK_SIZE = 65536

def analyze_observed_part(part_size, name):
    """Analyze an observed part size from the real failure"""
    print(f"\n{name}:")
    print(f"  Observed part size: {part_size:,} bytes ({part_size / (1024*1024):.2f} MB)")
    print(f"  Remainder mod 65536: {part_size % ARMOR_BLOCK_SIZE:,} bytes")
    print(f"  Remainder mod 512: {part_size % 512:,} bytes")
    print(f"  Is multiple of 512: {part_size % 512 == 0}")
    print(f"  Is multiple of 65536: {part_size % ARMOR_BLOCK_SIZE == 0}")

    # Check if this could be chunk_size + overflow
    # For uncompressed tar, overflow is typically < a few KB and multiple of 512
    # For compressed tar, overflow can be larger and unpredictable

    # Try to find a chunk_size that would produce this part
    # Assuming this is the first part (which pins P in ARMOR)
    # Or assuming it's a regular non-final part

    print(f"\n  Possible scenarios:")

    # Scenario 1: This is part 1, so chunk_size ≈ part_size (but slightly smaller)
    # The part flushed because buffer.tell() > chunk_size after a write
    # So chunk_size = part_size - N where N is the size of the last write

    if part_size % 512 == 0:
        # Could be uncompressed tar
        # Typical tar writes are 512 bytes, but can be larger
        # Let's assume the last write was a multiple of 512
        for last_write_size in [512, 1024, 2048, 4096, 8192, 16384]:
            if part_size > last_write_size:
                possible_chunk = part_size - last_write_size
                if possible_chunk % ARMOR_BLOCK_SIZE == 0:
                    print(f"    ✓ chunk_size = {possible_chunk:,} ({last_write_size:,}-byte write overflow)")
                    print(f"      This would be ARMOR-compliant!")

    # Scenario 2: Check if part_size itself is a valid chunk_size
    # (i.e., this part happens to be exactly chunk_size)
    if part_size % ARMOR_BLOCK_SIZE == 0:
        print(f"    ✓ Part size IS 65536-aligned (could be exact chunk_size)")
    else:
        print(f"    ✗ Part size is NOT 65536-aligned")

    # Scenario 3: This is from compression
    print(f"    • If using compression, part sizes are unpredictable")
    print(f"      and unlikely to be 65536-aligned")

def main():
    print("=" * 80)
    print("Real-World Failure Reconstruction")
    print("=" * 80)

    # Real observed part sizes from the incident
    queue_db_part = 11876352
    forgejo_postgres_part = 4284416

    analyze_observed_part(queue_db_part, "queue-db")
    analyze_observed_part(forgejo_postgres_part, "forgejo-postgres")

    print("\n" + "=" * 80)
    print("KEY INSIGHT")
    print("=" * 80)
    print("Both observed parts have remainders that are multiples of 512")
    print("but NOT multiples of 65536:")
    print()
    print(f"  queue-db: 11,876,352 = 65536 × 181 + 14,336")
    print(f"             14,336 = 512 × 28 (multiple of 512, not 65536)")
    print()
    print(f"  forgejo-postgres: 4,284,416 = 65536 × 65 + 24,576")
    print(f"                    24,576 = 512 × 48 (multiple of 512, not 65536)")
    print()
    print("This is consistent with barman's uncompressed tar behavior:")
    print("  - Tar writes in 512-byte blocks")
    print("  - Buffer flushes when size > chunk_size")
    print("  - Final part size = chunk_size + N×512 where N ≥ 0")
    print("  - ARMOR requires final part size = chunk_size + N×65536")
    print()
    print("THE MISMATCH: Tar uses 512-byte alignment, ARMOR needs 65536-byte alignment.")
    print("Barman cannot bridge this gap without changes.")

if __name__ == '__main__':
    main()
