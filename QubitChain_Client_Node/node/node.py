import asyncio
import logging
import uuid
import time
import json
from QubitChain.node.core.blockchain import Blockchain
from QubitChain.node.network.p2p import P2PNetwork
from QubitChain.node.quantum.miner import Miner
from QubitChain.node.network.messages import Message, MSG_TYPE

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

class QubitNode:
    def __init__(self, host="127.0.0.1", port=8000, miner_address="QubitMinerAddress"):
        self.node_id = str(uuid.uuid4())
        self.host = host
        self.port = port
        self.blockchain = Blockchain(genesis_path="../genesis.json")
        self.p2p = P2PNetwork(host, port, self.blockchain, self.node_id)
        self.miner = Miner(self.blockchain, miner_address)
        self.is_running = False
        self.loop = asyncio.get_event_loop()
        self.status_callback = None # Callback pour la GUI

    def set_status_callback(self, callback):
        """Définit la fonction de rappel pour mettre à jour l'état dans la GUI."""
        self.status_callback = callback

    def get_status(self):
        """Retourne l'état actuel du node pour la GUI."""
        latest_block = self.blockchain.get_latest_block()
        return {
            "is_running": self.is_running,
            "chain_height": self.blockchain.get_chain_height(),
            "total_supply": self.blockchain.get_total_supply(),
            "latest_hash": latest_block.hash if latest_block else "N/A",
            "connected_peers": self.p2p.peer_manager.get_peer_count(),
            "mining_logs": "Simulated Q-PoW logs...",
            "pending_tx": len(self.blockchain.pending_transactions)
        }

    def update_status(self):
        """Appelle le callback pour mettre à jour la GUI."""
        if self.status_callback:
            self.status_callback(self.get_status())

    async def start_node_async(self):
        """Démarre le serveur P2P et la boucle de synchronisation."""
        if self.is_running:
            logging.warning("Le node est déjà en cours d'exécution.")
            return

        self.is_running = True
        logging.info(f"Démarrage du node QubitChain ID: {self.node_id}")
        
        # Démarrer le serveur P2P
        server_task = self.loop.create_task(self.p2p.start_server())
        
        # Tâche de synchronisation et de ping
        sync_task = self.loop.create_task(self.sync_loop())
        
        # Tâche de mise à jour de la GUI
        gui_update_task = self.loop.create_task(self.gui_update_loop())

        await asyncio.gather(server_task, sync_task, gui_update_task)

    def start_node(self):
        """Fonction synchrone pour démarrer le node (appelée par la GUI)."""
        try:
            self.loop.run_until_complete(self.start_node_async())
        except KeyboardInterrupt:
            self.stop_node()
        except Exception as e:
            logging.error(f"Erreur fatale du node: {e}")
            self.stop_node()

    def stop_node(self):
        """Arrête le node et le serveur P2P."""
        self.is_running = False
        self.p2p.stop()
        logging.info("Node QubitChain arrêté.")
        # Arrêter toutes les tâches asyncio
        for task in asyncio.all_tasks(self.loop):
            task.cancel()

    async def sync_loop(self):
        """Boucle de synchronisation périodique (connexion aux pairs, demande de chaîne)."""
        while self.is_running:
            # 1. Se connecter aux pairs connus (pour l'exemple, on n'en a pas encore)
            # Pour un testnet, on pourrait avoir une liste de seed nodes
            
            # 2. Demander la chaîne aux pairs connectés
            if self.p2p.peer_manager.get_peer_count() > 0:
                logging.info("Synchronisation: Demande de chaîne aux pairs...")
                get_chain_msg = Message(MSG_TYPE["GET_CHAIN"])
                await self.p2p.broadcast(get_chain_msg)
            
            await asyncio.sleep(10) # Synchroniser toutes les 10 secondes

    async def gui_update_loop(self):
        """Boucle de mise à jour périodique de l'état pour la GUI."""
        while self.is_running:
            self.update_status()
            await asyncio.sleep(1) # Mettre à jour toutes les secondes

    def mine_block_sync(self):
        """Fonction synchrone pour miner un bloc (appelée par la GUI)."""
        if not self.is_running:
            logging.warning("Impossible de miner: le node n'est pas démarré.")
            return False

        # Simuler une transaction en attente pour le minage
        transactions_to_mine = self.blockchain.pending_transactions[:]
        
        # Exécuter le minage dans un thread ou un processus séparé si c'était bloquant
        # Ici, on suppose que le minage simulé est rapide ou géré par asyncio
        
        new_block = self.miner.mine_block(transactions_to_mine)
        
        if new_block:
            if self.blockchain.add_block(new_block):
                logging.info(f"Nouveau bloc {new_block.index} ajouté à la chaîne.")
                
                # Diffuser le nouveau bloc
                block_msg = Message(MSG_TYPE["NEW_BLOCK"], {"block": new_block.to_dict()})
                self.loop.create_task(self.p2p.broadcast(block_msg))
                
                self.update_status()
                return True
            else:
                logging.error("Échec de l'ajout du bloc miné à la chaîne.")
                return False
        else:
            logging.warning("Le minage n'a pas abouti.")
            return False

if __name__ == '__main__':
    # Exemple d'exécution du node (sans GUI)
    node = QubitNode(port=8000)
    
    # Pour le test, on ajoute une transaction en attente
    node.blockchain.pending_transactions.append({
        "from": "user_a",
        "to": "user_b",
        "amount": 10.0
    })
    
    print("Démarrage du node (Ctrl+C pour arrêter)...")
    
    # Démarrer le node dans un thread séparé pour permettre l'interaction
    # Dans l'environnement de la GUI, cela sera géré différemment.
    
    # Pour le test simple, on peut juste miner un bloc
    node.is_running = True # On force l'état pour le test
    node.mine_block_sync()
    
    print(f"\nStatut final: {node.get_status()}")
    
    # Pour un vrai test asynchrone, il faudrait utiliser un autre script
    # ou un environnement qui gère le loop asyncio.
    # node.start_node() # Ceci bloquerait le thread principal.
    
    node.is_running = False # Nettoyage pour le test
