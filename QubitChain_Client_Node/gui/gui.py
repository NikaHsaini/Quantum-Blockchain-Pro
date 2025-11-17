import sys
import threading
import time
from PySide6.QtWidgets import (
    QApplication, QMainWindow, QWidget, QVBoxLayout, QHBoxLayout, 
    QPushButton, QStatusBar, QMessageBox
)
from PySide6.QtCore import QTimer, Slot, Signal, QObject
from QubitChain.node.node import QubitNode
from QubitChain.gui.ui_components import StatusDisplay, MinedBlocksDisplay

# Classe pour gérer les signaux entre le thread du node et le thread de la GUI
class NodeSignals(QObject):
    status_updated = Signal(dict)
    node_started = Signal()
    node_stopped = Signal()
    block_mined = Signal(int, str)

class QubitChainGUI(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("QubitChain Node Client")
        self.setGeometry(100, 100, 800, 600)
        
        self.node = QubitNode(port=8000) # Node par défaut
        self.node_thread = None
        self.signals = NodeSignals()
        
        self._setup_ui()
        self._connect_signals()
        
        # Définir le callback du node vers la GUI
        self.node.set_status_callback(self._on_node_status_update)

    def _setup_ui(self):
        """Configure l'interface utilisateur."""
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        
        main_layout = QVBoxLayout(central_widget)
        
        # 1. Zone des boutons
        button_layout = QHBoxLayout()
        self.start_button = QPushButton("Start Node")
        self.stop_button = QPushButton("Stop Node")
        self.mine_button = QPushButton("Mine Block")
        
        self.stop_button.setEnabled(False)
        self.mine_button.setEnabled(False)
        
        button_layout.addWidget(self.start_button)
        button_layout.addWidget(self.stop_button)
        button_layout.addWidget(self.mine_button)
        button_layout.addStretch(1)
        
        main_layout.addLayout(button_layout)
        
        # 2. Affichage du statut
        self.status_display = StatusDisplay()
        main_layout.addWidget(self.status_display)
        
        # 3. Affichage des blocs minés
        self.blocks_display = MinedBlocksDisplay()
        main_layout.addWidget(self.blocks_display)
        
        main_layout.addStretch(1)
        
        # 4. Barre de statut
        self.status_bar = QStatusBar()
        self.setStatusBar(self.status_bar)
        self.status_bar.showMessage("Node arrêté. Cliquez sur 'Start Node' pour commencer.")

    def _connect_signals(self):
        """Connecte les signaux aux slots."""
        self.start_button.clicked.connect(self.start_node_clicked)
        self.stop_button.clicked.connect(self.stop_node_clicked)
        self.mine_button.clicked.connect(self.mine_block_clicked)
        
        # Connexion des signaux du thread du node au thread de la GUI
        self.signals.status_updated.connect(self.status_display.update_status)
        self.signals.node_started.connect(self._on_node_started)
        self.signals.node_stopped.connect(self._on_node_stopped)
        self.signals.block_mined.connect(self._on_block_mined)

    @Slot(dict)
    def _on_node_status_update(self, status_data):
        """Reçoit les mises à jour de statut du thread du node et les émet pour la GUI."""
        self.signals.status_updated.emit(status_data)
        
        # Mise à jour de la barre de statut
        if status_data.get("is_running"):
            self.status_bar.showMessage(f"Node en cours d'exécution. Hauteur: {status_data['chain_height']}, Pairs: {status_data['connected_peers']}")

    @Slot()
    def _on_node_started(self):
        """Met à jour l'état de la GUI après le démarrage du node."""
        self.start_button.setEnabled(False)
        self.stop_button.setEnabled(True)
        self.mine_button.setEnabled(True)
        self.status_bar.showMessage("Node QubitChain démarré et synchronisation en cours...")

    @Slot()
    def _on_node_stopped(self):
        """Met à jour l'état de la GUI après l'arrêt du node."""
        self.start_button.setEnabled(True)
        self.stop_button.setEnabled(False)
        self.mine_button.setEnabled(False)
        self.status_bar.showMessage("Node arrêté.")
        
        # Mettre à jour l'affichage du statut à l'état arrêté
        self.status_display.update_status(self.node.get_status())

    @Slot(int, str)
    def _on_block_mined(self, index, hash_val):
        """Met à jour l'affichage après le minage d'un bloc."""
        self.blocks_display.update_last_block(index, hash_val)
        QMessageBox.information(self, "Minage Réussi", f"Bloc {index} miné avec succès!")

    def start_node_clicked(self):
        """Démarre le node dans un thread séparé."""
        if self.node_thread is None or not self.node_thread.is_alive():
            self.node_thread = threading.Thread(target=self._run_node, daemon=True)
            self.node_thread.start()
            self.signals.node_started.emit()

    def _run_node(self):
        """Fonction cible pour le thread du node."""
        self.node.start_node()

    def stop_node_clicked(self):
        """Arrête le node."""
        if self.node_thread and self.node_thread.is_alive():
            self.node.stop_node()
            self.node_thread.join(timeout=5) # Attendre l'arrêt du thread
            self.signals.node_stopped.emit()

    def mine_block_clicked(self):
        """Déclenche le minage d'un bloc."""
        # Le minage est synchrone dans node.py, donc on le lance dans un thread pour ne pas bloquer la GUI
        def run_mining():
            success = self.node.mine_block_sync()
            if success:
                latest = self.node.blockchain.get_latest_block()
                self.signals.block_mined.emit(latest.index, latest.hash)
        
        mining_thread = threading.Thread(target=run_mining, daemon=True)
        mining_thread.start()

    def closeEvent(self, event):
        """Gère la fermeture de la fenêtre."""
        if self.node.is_running:
            self.node.stop_node()
            if self.node_thread and self.node_thread.is_alive():
                self.node_thread.join(timeout=5)
        event.accept()

def main():
    app = QApplication(sys.argv)
    window = QubitChainGUI()
    window.show()
    sys.exit(app.exec())

if __name__ == '__main__':
    # Pour exécuter la GUI, il faut un environnement graphique.
    # Dans le sandbox, on ne peut pas l'exécuter directement, mais on écrit le code.
    # main()
    print("Fichier GUI prêt. Utilise PySide6.")
