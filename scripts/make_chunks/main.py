import math
import struct
import numpy as np

from hierarchical_clustering import constrained_hierarchical_clustering



def bytes_to_int_list(buf: bytes) -> list:
    # Calculate how many 16-bit integers are in the buffer.
    n = len(buf) // 2
    # 'H' is the format for an unsigned 16-bit integer.
    return list(struct.unpack(f"{n}H", buf))

def i2p(idx, size):

    s2 = size * size

    return ( idx % size, int(idx / s2), int( (idx % s2 ) / size ) )

def expand(condensed):
    # We don't need to preallocate since we can use list append.
    expanded = []
    value = None

    # Process elements starting from index 2
    for i in range(2, len(condensed)):
        if (condensed[i] & 1) == 1:
            value = condensed[i] >> 1
            expanded.append(value)
        else:
            repeat = condensed[i] >> 1
            for _ in range(repeat):
                expanded.append(value)
    
    return expanded



# read build data file
with open("./0x00", "rb") as file:

    data = file.read()
    condensed = bytes_to_int_list(data)
    file.close()

expanded = expand(condensed)
points = []
build_size = condensed[1]
bs2 = build_size * build_size

for i in range(len(expanded)):

    pos = i2p(i, build_size)
    val = expanded[i]

    if val == 0 and (pos[1] == 0 or expanded[i - bs2] > 24):
        points.append([pos[0], pos[1], pos[2]])
    
points_array = np.array(points[0:10000])
clusters = constrained_hierarchical_clustering(points_array, max_cluster_size=6)

print(len(clusters))