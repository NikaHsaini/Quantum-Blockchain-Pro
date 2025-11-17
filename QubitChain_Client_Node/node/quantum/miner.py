import time
import logging
from QubitChain.node.core.block import Block
from QubitChain.node.core.consensus import get_difficulty, calculate_reward
from QubitChain.node.quantum.circuits import simulate_qpow_proof
from QubitChain.node.core.hashing import sha_666

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

class Miner:
    def __init__(self, blockchain, miner_address="QubitMinerAddress"):
        self.blockchain = blockchain
        self.miner_address = miner_address

    def mine_block(self, transactions):
        """
        Tente de miner un nouveau bloc en utilisant le Q-PoW simulé.
        """
        latest_block = self.blockchain.get_latest_block()
        if not latest_block:
            logging.error("Impossible de miner: pas de bloc précédent trouvé.")
            return None

        new_block_index = latest_block.index + 1
        difficulty = get_difficulty(new_block_index)
        
        # 1. Ajouter la récompense de minage aux transactions
        reward = calculate_reward(new_block_index)
        coinbase_tx = {
            "from": "coinbase",
            "to": self.miner_address,
            "amount": reward,
            "message": f"Block reward for block {new_block_index}"
        }
        mining_transactions = [coinbase_tx] + transactions
        
        # 2. Créer le bloc candidat
        new_block = Block(
            index=new_block_index,
            timestamp=time.time(),
            transactions=mining_transactions,
            previous_hash=latest_block.hash,
            nonce="0" # Le nonce sera incrémenté pendant le minage
        )
        
        logging.info(f"Démarrage du minage pour le bloc {new_block_index} avec difficulté {difficulty}...")
        
        # 3. Minage Q-PoW (simulation)
        # Le Q-PoW est basé sur la recherche d'un nonce qui, une fois haché,
        # produit un résultat qui satisfait la difficulté.
        # Dans la simulation, nous allons itérer sur le nonce et utiliser le 
        # simulateur quantique pour générer une "preuve" qui doit être incluse
        # dans le bloc pour valider le minage.
        
        target = "0" * difficulty
        nonce_counter = 0
        
        start_time = time.time()
        
        while True:
            new_block.nonce = str(nonce_counter)
            
            # Recalculer le hash du bloc avec le nouveau nonce
            block_string_for_hash = new_block.to_dict(include_hash=False)
            current_hash = sha_666(block_string_for_hash)
            
            # Simuler la preuve quantique (Q-PoW)
            # La preuve quantique est ici simplifiée pour être le hash lui-même
            # qui doit satisfaire la difficulté.
            
            # Dans un vrai Q-PoW, on utiliserait le simulateur pour trouver un nonce
            # qui maximise la probabilité de l'état cible (le hash valide).
            # Ici, on simule l'effort quantique par l'itération du nonce jusqu'à
            # ce que le hash classique satisfasse la difficulté.
            
            if current_hash.startswith(target):
                # Succès du minage
                new_block.hash = current_hash
                end_time = time.time()
                logging.info(f"Bloc miné avec succès! Nonce: {nonce_counter}, Hash: {current_hash[:10]}..., Temps: {end_time - start_time:.2f}s")
                
                # Ajout d'une preuve quantique simulée au bloc (pour la forme)
                # Dans une implémentation réelle, le résultat de la simulation serait ici
                new_block.transactions.append({"quantum_proof": "Simulated Q-PoW Success"})
                
                return new_block
            
            nonce_counter += 1
            
            # Limite pour éviter une boucle infinie dans un environnement de test
            if nonce_counter > 100000:
                logging.warning("Limite de nonce atteinte sans trouver de solution.")
                return None

# Le test de ce module sera fait dans node.py car il nécessite une instance de Blockchain.
