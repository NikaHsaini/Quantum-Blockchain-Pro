import time
import json
from QubitChain.node.core.hashing import sha_666

class Block:
    def __init__(self, index, timestamp, transactions, previous_hash, nonce=""):
        self.index = index
        self.timestamp = timestamp
        self.transactions = transactions
        self.previous_hash = previous_hash
        self.nonce = nonce
        self.hash = self.calculate_hash()

    def calculate_hash(self):
        """Calcule le hash SHA-666 du bloc."""
        block_string = json.dumps(self.to_dict(include_hash=False), sort_keys=True)
        return sha_666(block_string)

    def mine(self, difficulty, owner=""):
        """
        Simule le minage en trouvant un nonce qui satisfait la difficulté.
        Pour Q-PoW, on utilise une simulation simple basée sur le nonce.
        Le vrai minage quantique sera dans quantum/miner.py.
        """
        target = "0" * difficulty
        while self.hash[:difficulty] != target:
            self.nonce = str(int(self.nonce) + 1) if self.nonce.isdigit() else "0"
            self.hash = self.calculate_hash()
        
        # Pour le genesis block, on utilise le nonce spécial
        if self.index == 0:
            self.nonce = "GENESIS"
            self.hash = self.calculate_hash() # Recalculer avec le nonce GENESIS
            
        return self.hash

    def to_dict(self, include_hash=True):
        """Retourne une représentation du bloc sous forme de dictionnaire."""
        block_data = {
            "index": self.index,
            "timestamp": self.timestamp,
            "transactions": self.transactions,
            "previous_hash": self.previous_hash,
            "nonce": self.nonce,
        }
        if include_hash:
            block_data["hash"] = self.hash
        return block_data

    @classmethod
    def from_dict(cls, block_data):
        """Crée un objet Block à partir d'un dictionnaire."""
        block = cls(
            block_data["index"],
            block_data["timestamp"],
            block_data["transactions"],
            block_data["previous_hash"],
            block_data.get("nonce", "")
        )
        # Le hash est recalculé à l'initialisation, donc on ne le passe pas
        return block

# Exemple de création de bloc
if __name__ == '__main__':
    # Création d'un bloc de test
    test_block = Block(
        index=1,
        timestamp=time.time(),
        transactions=["tx1", "tx2"],
        previous_hash="0" * 128,
        nonce="0"
    )
    
    print("--- Bloc non miné ---")
    print(json.dumps(test_block.to_dict(), indent=4))
    
    # Minage simulé (difficulté 4)
    print("\n--- Minage (Difficulté 4) ---")
    test_block.mine(difficulty=4)
    print(json.dumps(test_block.to_dict(), indent=4))
    print(f"Hash valide: {test_block.hash[:4] == '0000'}")
