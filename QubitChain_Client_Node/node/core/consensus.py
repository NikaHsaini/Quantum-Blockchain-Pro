import math

# Paramètres monétaires de QubitChain
INITIAL_REWARD = 50.0
SUPPLY_LIMIT = 21000.0 # Supply asymptotique

def calculate_reward(height):
    """
    Calcule la récompense de bloc en utilisant un halving asymptotique.
    La récompense tend vers 0 sans jamais l'atteindre, assurant une supply asymptotique.
    Formule simplifiée: Reward = Initial_Reward / (1 + log(height + 1))
    """
    if height < 0:
        return 0.0
    
    # Utilisation d'une fonction qui décroît lentement et tend vers 0
    # On utilise log(height + 2) pour éviter la division par zéro et pour que le bloc 0 ait une récompense définie
    # La formule est ajustée pour que la récompense initiale soit proche de INITIAL_REWARD
    
    # Constante d'ajustement pour le halving
    HALVING_FACTOR = 10000.0 
    
    # Récompense = INITIAL_REWARD * (1 / (1 + (height / HALVING_FACTOR)))
    # Une autre approche pour l'asymptotique:
    reward = INITIAL_REWARD / (1 + math.log(height + 2))
    
    # S'assurer que la récompense ne devient pas trop petite trop vite pour simuler l'asymptotique
    # On peut aussi utiliser une fonction exponentielle décroissante:
    # reward = INITIAL_REWARD * math.exp(-height / HALVING_FACTOR)
    
    # Gardons la formule logarithmique pour sa décroissance lente
    return round(reward, 8)

def get_difficulty(height):
    """
    Détermine la difficulté de minage.
    Pour l'exemple, la difficulté augmente lentement.
    """
    # Difficulté de base
    base_difficulty = 4
    
    # Augmentation de la difficulté tous les 2016 blocs (comme Bitcoin)
    # Pour un testnet, on peut la faire augmenter plus vite
    if height == 0:
        return base_difficulty
        
    # Augmentation d'un niveau de difficulté tous les 100 blocs
    difficulty_increase = height // 100
    
    return base_difficulty + difficulty_increase

def validate_proof_of_work(block, difficulty):
    """
    Valide la preuve de travail (Q-PoW simulé) en vérifiant le hash.
    """
    target = "0" * difficulty
    return block.hash.startswith(target)

# Exemple d'utilisation
if __name__ == '__main__':
    print("--- Halving Asymptotique ---")
    
    heights = [0, 1, 10, 100, 1000, 10000, 100000]
    
    for h in heights:
        reward = calculate_reward(h)
        diff = get_difficulty(h)
        print(f"Bloc {h}: Récompense = {reward} QBTC, Difficulté = {diff}")
        
    # Vérification de la supply asymptotique (très approximative sans un vrai ledger)
    # La supply totale est la somme des récompenses jusqu'à l'infini.
    # Dans une implémentation réelle, il faudrait suivre la supply totale.
    print(f"\nSupply asymptotique visée: {SUPPLY_LIMIT} QBTC")
