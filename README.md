# QubitChain Client Node

Bienvenue dans le client node pour **QubitChain**, une blockchain expérimentale post-quantique. Ce projet implémente les spécifications uniques de QubitChain, notamment son algorithme de hachage propriétaire **SHA-666**, son mécanisme de consensus **Quantum Proof-of-Work (Q-PoW)** basé sur Grover, et son modèle économique à **halving asymptotique**.

## 1. Architecture du Projet

Le projet est structuré pour séparer clairement les préoccupations du *node* (logique métier de la blockchain) et de l'*interface graphique* (interaction utilisateur).

\`\`\`
/QubitChain
    /node
        /core               # Logique de base de la blockchain
            block.py        # Définition du bloc
            blockchain.py   # Gestion de la chaîne et validation
            consensus.py    # Règles de consensus (difficulté, récompense, halving)
            hashing.py      # Implémentation du SHA-666
        /network            # Communication P2P
            p2p.py          # Serveur et client WebSocket
            peers.py        # Gestion des pairs
            messages.py     # Définition des messages réseau
        /quantum            # Logique quantique (simulation)
            circuits.py     # Circuits quantiques Qiskit (simulés)
            miner.py        # Algorithme de minage Q-PoW (simulé)
        node.py             # Point d'entrée et contrôleur principal du node

    /gui                    # Interface utilisateur graphique
        gui.py              # Fenêtre principale et logique de la GUI (PySide6)
        ui_components.py    # Widgets personnalisés pour l'affichage

    genesis.json            # Bloc de genèse de QubitChain
    README.md               # Ce document
    requirements.txt        # Liste des dépendances Python
    build_zip.sh            # Script pour créer l'archive ZIP
\`\`\`

## 2. Fonctionnement Technique

### SHA-666
L'algorithme de hachage propriétaire **SHA-666** est défini dans \`node/core/hashing.py\`. Il s'agit d'un triple hachage SHA3-512 : $H(x) = SHA3-512(SHA3-512(SHA3-512(x)))$.

### Quantum Proof-of-Work (Q-PoW)
Le consensus Q-PoW est simulé en utilisant la librairie **Qiskit** (\`node/quantum/circuits.py\` et \`node/quantum/miner.py\`). Bien que l'exécution réelle d'un algorithme de Grover pour le minage soit complexe et coûteuse en ressources, l'implémentation simule le processus en utilisant un circuit quantique simplifié pour illustrer le concept. Le minage consiste à trouver un nonce qui satisfait la difficulté classique (hash commençant par $N$ zéros) tout en intégrant une preuve de travail quantique simulée.

### Halving Asymptotique
Le modèle monétaire est basé sur une **supply asymptotique de 21 000 QBTC**. La récompense de bloc est calculée via une fonction logarithmique décroissante (\`node/core/consensus.py\`) qui tend vers zéro sans jamais l'atteindre, assurant que la supply totale reste toujours inférieure à la limite asymptotique.

## 3. Prérequis et Installation

Ce projet nécessite **Python 3.12+** et les dépendances listées dans \`requirements.txt\`.

1.  **Cloner le dépôt (ou dézipper l'archive)**
    \`\`\`bash
    unzip QubitChain_Client_Node.zip
    cd QubitChain
    \`\`\`

2.  **Installer les dépendances**
    \`\`\`bash
    pip install -r requirements.txt
    \`\`\`

## 4. Lancement du Node et de la GUI

Le node et la GUI sont conçus pour fonctionner ensemble.

### Comment lancer la GUI (Recommandé)

La GUI est le point d'entrée recommandé. Elle gère le cycle de vie du node.

\`\`\`bash
python3 main.py
\`\`\`

Une fois la fenêtre ouverte, cliquez sur le bouton **"Start Node"**. Cela démarrera le serveur P2P et la boucle de synchronisation dans un thread séparé.

### Comment lancer un Node (pour un testnet)

Pour simuler un testnet, vous devez lancer plusieurs instances du node sur des ports différents.

1.  **Node 1 (Port 8000)**
    \`\`\`bash
    python3 -m node.node 8000
    \`\`\`
    *(Note: Pour une exécution en ligne de commande pure, vous devrez modifier \`node.py\` pour gérer l'exécution asynchrone sans la GUI.)*

2.  **Node 2 (Port 8001)**
    \`\`\`bash
    python3 -m node.node 8001
    \`\`\`
    *(Vous devrez ajouter manuellement l'adresse du Node 1 (ws://127.0.0.1:8000) à la liste des pairs connus du Node 2 pour qu'ils se connectent.)*

### Comment se connecter au testnet

Le node tente de se connecter aux pairs connus via le module \`node/network/p2p.py\`. Pour un testnet local, vous pouvez modifier la liste des pairs connus dans \`node/network/peers.py\` ou implémenter une fonction de découverte de pairs.

### Comment miner

Le minage est déclenché via le bouton **"Mine Block"** dans l'interface graphique.

1.  Cliquez sur **"Start Node"**.
2.  Cliquez sur **"Mine Block"**.
3.  Le node tentera de trouver un nonce valide en utilisant la simulation Q-PoW.
4.  Une fois le bloc miné, il sera ajouté à la chaîne locale et diffusé aux pairs connectés. Le statut de la GUI sera mis à jour.

## 5. Livrables

Ce projet est livré avec :
1.  Tous les fichiers Python et le \`genesis.json\`.
2.  Le fichier ZIP complet créé par \`build_zip.sh\`.
3.  Ce \`README.md\` documenté.

---
*Projet développé par Manus AI pour QubitChain.*
\`\`\`
