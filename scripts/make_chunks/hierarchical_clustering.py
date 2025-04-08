import numpy as np
from scipy.spatial import KDTree

def constrained_hierarchical_clustering(points, max_cluster_size=8):
    """
    Perform constrained hierarchical clustering on a set of 3D points.
    
    Each cluster is merged with its nearest neighbor (by centroid distance)
    only if the merge does not result in more than max_cluster_size points.
    
    Parameters:
      points (np.ndarray): Array of shape (n_points, 3) with 3D coordinates.
      max_cluster_size (int): Maximum allowed number of points in a cluster.
      
    Returns:
      clusters (list of lists): Each sub-list contains the indices of points
                                that belong to that cluster.
    """
    n_points = points.shape[0]
    # Initialize each point as its own cluster (store indices) and the centroids.
    clusters = [[i] for i in range(n_points)]
    centroids = points.copy()  # Initially each centroid is just the point

    merged = True
    iteration = 0
    while merged:
        iteration += 1
        merged = False
        # Build KDTree for the current centroids for fast nearest neighbor lookup.
        tree = KDTree(centroids)
        merge_candidates = []
        
        # For each cluster, find its nearest neighbor (other than itself)
        for i, centroid in enumerate(centroids):
            # Query the 2 nearest points (first is the point itself)
            dists, idxs = tree.query(centroid, k=2)
            neighbor = idxs[1]
            distance = dists[1]
            # Only consider merging if combined size is within the allowed max
            if len(clusters[i]) + len(clusters[neighbor]) <= max_cluster_size:
                merge_candidates.append((distance, i, neighbor))
        
        # If no valid merge candidates, then we're done.
        if not merge_candidates:
            break
        
        # Sort by distance to choose the closest merge candidate.
        merge_candidates.sort(key=lambda x: x[0])
        distance, i, j = merge_candidates[0]
        
        # Merge clusters i and j (always merge into the lower-index cluster)
        new_cluster = clusters[i] + clusters[j]
        new_centroid = np.mean(points[new_cluster], axis=0)
        clusters[i] = new_cluster
        centroids[i] = new_centroid
        
        # Remove cluster j and its centroid from the lists.
        # NOTE: Removing by index shifts subsequent indices; to keep things
        # simple we remove j (assuming j > i, which is usually the case for 
        # the nearest neighbor query, but if not, you may need to adjust).
        clusters.pop(j)
        centroids = np.delete(centroids, j, axis=0)
        
        merged = True  # A merge occurred; continue looping.
        print(f"Iteration {iteration}: Merged clusters {i} and {j}, new cluster size = {len(new_cluster)}")
    
    return clusters

