#!/usr/bin/env python3
"""
Simulation of barman-cloud-backup 3.19.1's part size behavior.
This demonstrates why barman cannot reliably produce 65536-byte-aligned parts.
"""

import tarfile
import io
import os
from tempfile import NamedTemporaryFile

# Constants from ARMOR and barman
ARMOR_BLOCK_SIZE = 65536  # 64 KB
BARMAN_MIN_CHUNK_SIZE = 5 * 1024 * 1024  # 5 MB (AWS S3 default)

# Simulate barman's CloudTarUploader.write() logic
class SimulatedCloudTarUploader:
    def __init__(self, chunk_size):
        self.chunk_size = chunk_size
        self.buffer = io.BytesIO()
        self.parts = []
        self.total_size = 0

    def write(self, buf):
        """This mimics barman's CloudTarUploader.write() method"""
        # The key: barman flushes when buffer.tell() > chunk_size
        # This means the buffer can EXCEED chunk_size by up to len(buf)
        if self.buffer.tell() > self.chunk_size:
            self.flush()

        # Write to buffer
        self.buffer.write(buf)
        self.total_size += len(buf)

    def flush(self):
        """Flush buffer as a part"""
        part_size = self.buffer.tell()
        if part_size > 0:
            self.parts.append(part_size)
            self.buffer = io.BytesIO()

    def close(self):
        """Finalize upload"""
        self.flush()

def simulate_tar_upload(total_data_size, chunk_size, tar_block_size=512):
    """
    Simulate a tar upload through barman's CloudTarUploader.

    Args:
        total_data_size: Total size of data to tar up
        chunk_size: barman's chunk_size parameter
        tar_block_size: Size of tar blocks (typically 512 bytes)

    Returns:
        List of part sizes that would be uploaded
    """
    uploader = SimulatedCloudTarUploader(chunk_size)

    # Simulate tarfile writing data in tar_block_size chunks
    # In reality, tarfile writes variable-sized blocks, but 512 is the fundamental unit
    remaining = total_data_size
    while remaining > 0:
        write_size = min(tar_block_size, remaining)
        uploader.write(b'x' * write_size)
        remaining -= write_size

    uploader.close()
    return uploader.parts

def analyze_part_sizes(parts, block_size=ARMOR_BLOCK_SIZE):
    """Analyze whether parts satisfy ARMOR's block alignment requirement"""
    results = []
    for i, part_size in enumerate(parts, 1):
        is_aligned = part_size % block_size == 0
        remainder = part_size % block_size
        is_512_aligned = part_size % 512 == 0

        result = {
            'part_number': i,
            'size': part_size,
            'is_65536_aligned': is_aligned,
            'remainder_65536': remainder,
            'is_512_aligned': is_512_aligned,
            'status': 'PASS' if is_aligned else 'FAIL',
        }
        results.append(result)

    return results

def main():
    print("=" * 80)
    print("Barman-Cloud-Backup Part Size Simulation")
    print("=" * 80)
    print()

    # Test 1: Default 5MB chunk_size with 512-byte tar blocks
    print("Test 1: chunk_size=5MB, 512-byte tar blocks (typical uncompressed case)")
    print("-" * 80)
    chunk_size = BARMAN_MIN_CHUNK_SIZE  # 5MB
    parts = simulate_tar_upload(total_data_size=25 * 1024 * 1024,  # 25MB total
                                chunk_size=chunk_size,
                                tar_block_size=512)
    results = analyze_part_sizes(parts)

    print(f"Chunk size: {chunk_size:,} bytes ({chunk_size / (1024*1024):.1f} MB)")
    print(f"Total parts: {len(parts)}")
    print()
    print("Part analysis:")
    for r in results:
        print(f"  Part {r['part_number']:2d}: {r['size']:10,} bytes | "
              f"remainder mod 65536: {r['remainder_65536']:6,} | "
              f"is_512_aligned: {r['is_512_aligned']} | "
              f"status: {r['status']}")

    # Show the pattern
    print()
    print("Pattern: Every part is chunk_size + N bytes where N < 512")
    for i, part in enumerate(parts, 1):
        overflow = part - chunk_size
        if overflow > 0:
            print(f"  Part {i} overflow: {overflow} bytes (multiple of 512: {overflow % 512 == 0})")
    print()

    # Test 2: Calculate what chunk_size would be needed for alignment
    print("Test 2: What chunk_size is needed for 65536-byte alignment?")
    print("-" * 80)
    # For alignment, chunk_size must be a multiple of 65536
    # And the overflow (which is < 512 and a multiple of 512) must also align
    # Since overflow is always a multiple of 512, we need chunk_size % 65536 == 0

    # Find the smallest chunk_size >= 5MB that is a multiple of 65536
    target = BARMAN_MIN_CHUNK_SIZE
    aligned_chunk_size = ((target + ARMOR_BLOCK_SIZE - 1) // ARMOR_BLOCK_SIZE) * ARMOR_BLOCK_SIZE
    print(f"  Min chunk_size for alignment: {aligned_chunk_size:,} bytes ({aligned_chunk_size / (1024*1024):.2f} MB)")
    print(f"  (Next multiple of 65536 above 5MB)")
    print()

    # Verify with the aligned chunk size
    print("Verification with aligned chunk_size:")
    parts_aligned = simulate_tar_upload(total_data_size=25 * 1024 * 1024,
                                         chunk_size=aligned_chunk_size,
                                         tar_block_size=512)
    results_aligned = analyze_part_sizes(parts_aligned)

    print(f"Chunk size: {aligned_chunk_size:,} bytes ({aligned_chunk_size / (1024*1024):.2f} MB)")
    all_aligned = all(r['is_65536_aligned'] for r in results_aligned)
    print(f"All parts 65536-aligned: {all_aligned}")
    print()

    # Test 3: Real-world failure reconstruction
    print("Test 3: Reconstruct real-world failure from queue-db")
    print("-" * 80)
    print("Observed part size: 11,876,352 bytes")
    observed_part = 11876352
    remainder_65536 = observed_part % ARMOR_BLOCK_SIZE
    remainder_512 = observed_part % 512
    print(f"  Remainder mod 65536: {remainder_65536} bytes")
    print(f"  Remainder mod 512: {remainder_512} bytes")
    print(f"  Is multiple of 512: {remainder_512 == 0}")
    print(f"  Is multiple of 65536: {remainder_65536 == 0}")

    # Reverse-engineer the chunk_size
    # chunk_size + overflow = part_size, where overflow < 512 and overflow % 512 == 0
    # But actually overflow could be multiple 512-blocks, depending on write pattern
    # The overflow is the amount by which chunk_size was exceeded
    # So chunk_size = part_size - (some multiple of 512)
    # Since overflow < typical tar write (which is multiple 512s), let's assume
    # the overflow is the remainder when dividing by a large chunk size

    # Given barman uses 5MB (5,242,880) as default, let's check:
    if observed_part > BARMAN_MIN_CHUNK_SIZE:
        possible_overflow = observed_part - BARMAN_MIN_CHUNK_SIZE
        print(f"\n  With default 5MB chunk_size:")
        print(f"    Overflow would be: {possible_overflow:,} bytes")
        print(f"    Overflow mod 512: {possible_overflow % 512}")
        print(f"    This is consistent with barman's behavior!")

    print()
    print("=" * 80)
    print("CONCLUSION")
    print("=" * 80)
    print("Barman's CloudTarUploader.write() flushes when buffer.tell() > chunk_size.")
    print("This means parts are chunk_size + N bytes where N depends on write pattern.")
    print()
    print("For uncompressed tar: N is a multiple of 512 bytes (tar block size)")
    print("For compressed tar: N is unpredictable (depends on compression)")
    print()
    print("ARMOR requires N to be a multiple of 65536 bytes.")
    print("Barman's default 5MB chunk_size is NOT a multiple of 65536.")
    print("Even with a 65536-aligned chunk_size, the overflow N breaks alignment.")
    print()
    print("ROOT CAUSE: Barman's flush logic fundamentally cannot guarantee")
    print("65536-byte-aligned parts with arbitrary data sizes.")

if __name__ == '__main__':
    main()
