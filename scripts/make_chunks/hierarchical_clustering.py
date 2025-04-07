import cupy as cp
import numpy as np
from cuml.neighbors import NearestNeighbors  # GPU-accelerated nearest neighbors

def constrained_hierarchical_clustering_gpu(points, max_cluster_size=6):
    """
    Perform constrained hierarchical clustering using GPU acceleration via CuPy and cuML.
    The function expects a NumPy array `points` of shape (n_points, 3).
    
    Parameters:
      points (np.ndarray): Array of shape (n_points, 3)
      max_cluster_size (int): Maximum allowed number of points in a cluster.
      
    Returns:
      clusters (list of lists): Each sub-list contains the indices of points that form that cluster.
    """
    # Move the dataset to the GPU using CuPy
    points_gpu = cp.asarray(points)
    n_points = points_gpu.shape[0]
    
    # Initialize: each point starts as its own cluster.
    clusters = [[i] for i in range(n_points)]
    
    # Initially, each centroid is just the corresponding point.
    centroids_gpu = points_gpu.copy()
    
    iteration = 0
    merged = True
    
    while merged:
        iteration += 1
        merged = False

        # Transfer centroids to CPU as cuML's NearestNeighbors currently works on NumPy arrays.
        centroids_np = cp.asnumpy(centroids_gpu)
        
        # Use cuML's GPU-accelerated NearestNeighbors to find the 2 nearest centroids for each centroid
        nn = NearestNeighbors(n_neighbors=2)
        nn.fit(centroids_np)
        distances, indices = nn.kneighbors(centroids_np)
        
        merge_candidates = []
        for i in range(len(clusters)):
            neighbor = indices[i, 1]  # First neighbor is itself, so take the second (nearest external)
            distance = distances[i, 1]
            
            # Check if merging cluster i and its neighbor stays within the allowed max size.
            if len(clusters[i]) + len(clusters[neighbor]) <= max_cluster_size:
                merge_candidates.append((distance, i, neighbor))
        
        # If no valid candidates, break out of the loop.
        if not merge_candidates:
            break

        # Sort candidates by distance (smallest first).
        merge_candidates.sort(key=lambda x: x[0])
        _, i, j = merge_candidates[0]
        
        # Merge clusters i and j.
        new_cluster = clusters[i] + clusters[j]
        clusters[i] = new_cluster

        # Update the centroid for the merged cluster using the original points.
        merged_points = cp.asarray(points[np.array(new_cluster)])
        new_centroid = cp.mean(merged_points, axis=0)
        centroids_gpu[i] = new_centroid

        # Remove cluster j and its centroid.
        clusters.pop(j)
        centroids_gpu = cp.delete(centroids_gpu, j, axis=0)
        
        merged = True
        print(f"Iteration {iteration}: merged clusters {i} and {j}, new cluster size = {len(new_cluster)}")

    return clusters

# Example usage:
if __name__ == '__main__':
    # For demonstration, create a random set of points.
    # Replace this with your actual data (e.g., np.random.rand(40000, 3)) for 40k points.
    np.random.seed(42)
    n_demo_points = 1000  # Change this to 40000 for your actual dataset.
    demo_points = np.random.rand(n_demo_points, 3)
    
    clusters = constrained_hierarchical_clustering_gpu(demo_points, max_cluster_size=6)
    print(f"Total clusters formed: {len(clusters)}")
