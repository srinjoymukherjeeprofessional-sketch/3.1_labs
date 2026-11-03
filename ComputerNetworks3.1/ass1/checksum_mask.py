def checksum(data):
    sum_val = 0
    pos = 0
    while pos + 1 < len(data):
        word = (data[pos] << 8) | data[pos + 1]
        sum_val += word
        sum_val = (sum_val & 0xffff) + (sum_val >> 16)
        pos += 2
    if pos < len(data):
        sum_val += data[pos] << 8
        sum_val = (sum_val & 0xffff) + (sum_val >> 16)
    while sum_val >> 16:
        sum_val = (sum_val & 0xffff) + (sum_val >> 16)
    return (~sum_val) & 0xffff

payload = b"ThisTestPayloadHasExactlyFortyFourBytesInIt!"
orig_csum = checksum(payload)

mask = bytearray(64)
found = False
for i in range(len(payload)):
    if found: break
    for j in range(i+2, len(payload), 2):
        if found: break
        for b in range(8):
            bit_i = (payload[i] >> b) & 1
            bit_j = (payload[j] >> b) & 1
            if bit_i != bit_j:
                mask[16 + i] ^= (1 << b)
                mask[16 + j] ^= (1 << b)
                found = True
                break

print(f"Mask found: {mask.hex()}")

corrupted = bytearray(payload)
for i in range(len(payload)):
    corrupted[i] ^= mask[16+i]

print(f"Original Checksum: {orig_csum:04x}")
print(f"Corrupted Checksum: {checksum(corrupted):04x}")
