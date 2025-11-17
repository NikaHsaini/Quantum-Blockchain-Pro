import json
import time
from QubitChain.node.core.block import Block
from QubitChain.node.core.hashing import sha_666
from QubitChain.node.core.consensus import calculate_reward, get_difficulty

class Blockchain:
    def __init__(self, genesis_path="genesis.json"):
        self.chain = []
        self.pending_transactions = []
        self.difficulty = 4 # Difficulté initiale
        self.genesis_path = genesis_path
        self.load_genesis_block()
        
    def load_genesis_block(self):
        """Charge le bloc de genèse à partir du fichier JSON."""
        try:
            with open(self.genesis_path, 'r') as f:
                genesis_data = json.load(f)
            
            # Le genesis block est un cas spécial
            genesis_block = Block.from_dict(genesis_data)
            genesis_block.hash = genesis_data["hash"] # On utilise le hash pré-calculé
            
            if genesis_block.index != 0:
                raise ValueError("Le bloc de genèse doit avoir l'index 0.")
                
            self.chain.append(genesis_block)
            print("Bloc de genèse chargé avec succès.")
            
        except FileNotFoundError:
            print(f"ATTENTION: Fichier de genèse non trouvé à {self.genesis_path}. La chaîne est vide.")
        except Exception as e:
            print(f"Erreur lors du chargement du bloc de genèse: {e}")

    def get_latest_block(self):
        """Retourne le dernier bloc de la chaîne."""
        return self.chain[-1] if self.chain else None

    def add_block(self, new_block):
        """Ajoute un nouveau bloc à la chaîne après validation."""
        if not self.is_valid_new_block(new_block, self.get_latest_block()):
            return False
            
        self.chain.append(new_block)
        self.pending_transactions = [] # Les transactions sont incluses dans le bloc
        self.difficulty = get_difficulty(len(self.chain)) # Mise à jour de la difficulté
        return True

    def is_valid_new_block(self, new_block, previous_block):
        """Valide un nouveau bloc."""
        if previous_block.index + 1 != new_block.index:
            print("Validation échouée: Index de bloc invalide.")
            return False
        
        if previous_block.hash != new_block.previous_hash:
            print("Validation échouée: Hash précédent invalide.")
            return False
            
        # Recalculer le hash pour vérifier l'intégrité
        if new_block.calculate_hash() != new_block.hash:
            print("Validation échouée: Hash du bloc invalide.")
            return False
            
        # Vérifier la preuve de travail (simulée)
        # Note: La vraie validation Q-PoW serait plus complexe
        target = "0" * get_difficulty(new_block.index)
        if not new_block.hash.startswith(target):
            print("Validation échouée: Preuve de travail invalide.")
            return False
            
        return True

    def is_chain_valid(self, chain):
        """Vérifie l'intégrité de toute la chaîne."""
        if not chain:
            return True
            
        # Vérifier le genesis block
        if chain[0].to_dict(include_hash=False) != self.chain[0].to_dict(include_hash=False):
            return False # Le genesis block doit être le même
            
        for i in range(1, len(chain)):
            current_block = chain[i]
            previous_block = chain[i-1]
            
            if not self.is_valid_new_block(current_block, previous_block):
                return False
                
        return True

    def replace_chain(self, new_chain):
        """Remplace la chaîne locale par une chaîne plus longue et valide."""
        if len(new_chain) > len(self.chain) and self.is_chain_valid(new_chain):
            self.chain = new_chain
            self.difficulty = get_difficulty(len(self.chain))
            print("Chaîne remplacée par une chaîne plus longue et valide.")
            return True
        return False

    def get_chain_height(self):
        """Retourne la hauteur de la chaîne (nombre de blocs)."""
        return len(self.chain)

    def get_total_supply(self):
        """Calcule la supply totale en sommant les récompenses de bloc."""
        total_supply = 0.0
        for i in range(len(self.chain)):
            # Le genesis block n'a pas de récompense de minage
            if i > 0:
                total_supply += calculate_reward(i)
        return round(total_supply, 8)

# Exemple d'utilisation (nécessite un genesis.json)
if __name__ == '__main__':
    # Ceci ne fonctionnera pas sans le fichier genesis.json
    print("Test de la classe Blockchain (nécessite genesis.json)")
    # blockchain = Blockchain()
    # print(f"Hauteur de la chaîne: {blockchain.get_chain_height()}")
    # print(f"Supply totale: {blockchain.get_total_supply()}")
