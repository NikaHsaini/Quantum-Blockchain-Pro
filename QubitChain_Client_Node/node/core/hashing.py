import hashlib
import json

def sha3_512(data):
    """Calcule le hash SHA3-512 des données fournies."""
    if isinstance(data, str):
        data = data.encode('utf-8')
    elif isinstance(data, dict):
        # Normaliser le dictionnaire pour un hachage cohérent
        data = json.dumps(data, sort_keys=True).encode('utf-8')
    
    return hashlib.sha3_512(data).hexdigest()

def sha_666(data):
    """
    Calcule le hash propriétaire SHA-666 (triple SHA3-512).
    SHA-666(data) = SHA3-512(SHA3-512(SHA3-512(data)))
    """
    # Première passe
    h1 = sha3_512(data)
    # Deuxième passe
    h2 = sha3_512(h1)
    # Troisième passe
    h3 = sha3_512(h2)
    
    return h3

if __name__ == '__main__':
    test_data = "QubitChain Test Data"
    hash_result = sha_666(test_data)
    print(f"Données de test: {test_data}")
    print(f"Hash SHA-666: {hash_result}")
    print(f"Longueur du hash: {len(hash_result)}")
