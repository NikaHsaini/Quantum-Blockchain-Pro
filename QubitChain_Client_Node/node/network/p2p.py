import asyncio
import websockets
import json
import logging
from QubitChain.node.network.peers import PeerManager
from QubitChain.node.network.messages import Message, MSG_TYPE

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

class P2PNetwork:
    def __init__(self, host, port, blockchain, node_id):
        self.host = host
        self.port = port
        self.blockchain = blockchain
        self.node_id = node_id
        self.peer_manager = PeerManager()
        self.server = None
        self.running = False

    async def start_server(self):
        """Démarre le serveur WebSocket pour écouter les connexions entrantes."""
        self.running = True
        try:
            self.server = await websockets.serve(self.handle_connection, self.host, self.port)
            logging.info(f"Serveur P2P démarré sur ws://{self.host}:{self.port}")
            await self.server.wait_closed()
        except Exception as e:
            logging.error(f"Erreur lors du démarrage du serveur P2P: {e}")
        finally:
            self.running = False

    async def connect_to_peer(self, uri):
        """Se connecte à un pair existant."""
        if uri in self.peer_manager.get_connected_peers_addresses():
            logging.warning(f"Déjà connecté à {uri}")
            return

        try:
            websocket = await websockets.connect(uri)
            logging.info(f"Connecté à un pair: {uri}")
            
            # Envoi du message de handshake
            handshake_msg = Message(MSG_TYPE["HANDSHAKE"], {"node_id": self.node_id, "height": self.blockchain.get_chain_height()})
            await websocket.send(handshake_msg.to_json())
            
            self.peer_manager.add_connected_peer(uri, websocket)
            
            # Démarrer la boucle de réception des messages
            await self.handle_connection(websocket, uri)
            
        except Exception as e:
            logging.error(f"Impossible de se connecter au pair {uri}: {e}")
            self.peer_manager.remove_connected_peer(uri)

    async def handle_connection(self, websocket, path=None):
        """Gère une connexion entrante ou sortante."""
        peer_uri = f"{websocket.remote_address[0]}:{websocket.remote_address[1]}"
        
        # Si c'est une connexion entrante, on l'ajoute après le handshake
        if path is not None:
            self.peer_manager.add_connected_peer(peer_uri, websocket)
            logging.info(f"Nouvelle connexion entrante de {peer_uri}")

        try:
            async for message in websocket:
                await self.handle_message(websocket, message)
        except websockets.exceptions.ConnectionClosedOK:
            logging.info(f"Connexion fermée avec {peer_uri}")
        except Exception as e:
            logging.error(f"Erreur de connexion avec {peer_uri}: {e}")
        finally:
            self.peer_manager.remove_connected_peer(peer_uri)

    async def handle_message(self, websocket, message_json):
        """Traite un message reçu."""
        msg = Message.from_json(message_json)
        
        if msg.type == MSG_TYPE["HANDSHAKE"]:
            # Répondre avec un ping et potentiellement demander la chaîne si elle est plus longue
            logging.info(f"Handshake reçu de Node ID: {msg.payload.get('node_id')}, Hauteur: {msg.payload.get('height')}")
            # Logique de synchronisation à implémenter dans node.py

        elif msg.type == MSG_TYPE["NEW_BLOCK"]:
            # Logique de validation et d'ajout de bloc
            logging.info(f"Nouveau bloc reçu: {msg.payload.get('block').get('index')}")
            # La validation et l'ajout seront gérés par le Node principal

        elif msg.type == MSG_TYPE["GET_CHAIN"]:
            # Envoyer la chaîne complète ou une partie
            logging.info("Demande de chaîne reçue. Envoi de la chaîne...")
            chain_data = [block.to_dict() for block in self.blockchain.chain]
            response = Message(MSG_TYPE["CHAIN_RESPONSE"], {"chain": chain_data})
            await websocket.send(response.to_json())

        elif msg.type == MSG_TYPE["CHAIN_RESPONSE"]:
            # Logique de remplacement de chaîne
            logging.info("Réponse de chaîne reçue. Tentative de synchronisation...")
            # La logique de remplacement sera gérée par le Node principal

        elif msg.type == MSG_TYPE["GET_PEERS"]:
            # Envoyer la liste des pairs connus
            peers_list = self.peer_manager.get_known_peers()
            response = create_peers_response_message(peers_list)
            await websocket.send(response.to_json())

        elif msg.type == MSG_TYPE["PEERS_RESPONSE"]:
            # Ajouter les nouveaux pairs à la liste des pairs connus
            new_peers = msg.payload.get("peers", [])
            for peer in new_peers:
                self.peer_manager.add_known_peer(peer)
            logging.info(f"Pairs mis à jour. Total connu: {len(self.peer_manager.known_peers)}")

        elif msg.type == MSG_TYPE["PING"]:
            # Répondre avec un PONG
            await websocket.send(Message(MSG_TYPE["PONG"]).to_json())

        elif msg.type == MSG_TYPE["PONG"]:
            logging.debug("PONG reçu.")

        else:
            logging.warning(f"Type de message inconnu: {msg.type}")

    async def broadcast(self, message):
        """Diffuse un message à tous les pairs connectés."""
        if not self.running:
            logging.warning("Le réseau P2P n'est pas démarré.")
            return

        message_json = message.to_json()
        
        # Utiliser asyncio.gather pour envoyer les messages en parallèle
        send_tasks = [
            peer.send(message_json) 
            for peer in self.peer_manager.connected_peers.values()
        ]
        
        if send_tasks:
            await asyncio.gather(*send_tasks, return_exceptions=True)
            logging.info(f"Message de type {message.type} diffusé à {len(send_tasks)} pairs.")

    def stop(self):
        """Arrête le serveur P2P."""
        if self.server:
            self.server.close()
            logging.info("Serveur P2P arrêté.")
        self.running = False

# Le test de ce module sera fait dans node.py car il nécessite une instance de Blockchain.
