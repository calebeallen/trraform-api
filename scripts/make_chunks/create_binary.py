import struct

# Read and parse the file
with open("./output1.txt", "r") as f:
    lines = f.readlines()

# Convert to list of uint32 integers
values = []
for line in lines:
    key, value = map(int, line.strip().split(":"))
    values.append(key)
    values.append(value)

# Convert to bytes
binary_data = b''.join(struct.pack('<I', v) for v in values)

# Write to a binary file
with open("chunk_map.dat", "wb") as f:
    f.write(binary_data)
