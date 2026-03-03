# Dossier de Compatibilité CBDC / Euro Numérique

**Auteur** : QUBITCOIN Foundation — Nika Hsaini
**Version** : 1.0.0
**Date** : 3 mars 2026

## 1. Vision Stratégique

L'intégration de QUBITCOIN avec l'écosystème de l'Euro Numérique est une pierre angulaire de notre stratégie institutionnelle. Elle vise à positionner QBTC comme l'actif de collatéral post-quantique de référence et le principal pont entre la finance traditionnelle (TradFi) et la finance décentralisée (DeFi) au sein de l'Union Européenne.

Notre approche garantit une conformité totale avec les cadres réglementaires **MiCA, eIDAS 2.0 et DORA**, tout en offrant une liquidité profonde et une stabilité de prix pour le QBTC.

## 2. Architecture Technique

L'architecture repose sur trois piliers contractuels :

| Contrat | Fichier | Rôle |
| :--- | :--- | :--- |
| **EuroDigitalBridge** | `EuroDigitalBridge.sol` | Pont bidirectionnel entre l'Euro Numérique (via DL3S/TARGET2) et sa représentation ERC-20 (wEURd) sur QUBITCOIN. |
| **QBTCLiquidityPool** | `QBTCLiquidityPool.sol` | Pool de liquidités institutionnelle (AMM) pour la paire QBTC/wEURd, avec liquidité concentrée, gestion algorithmique de liquidité et rééquilibrage dynamique TWAP. |
| **CBDCRouter** | `CBDCRouter.sol` | Routeur d'interopérabilité multi-CBDC, gérant les swaps entre QBTC, wEURd, et d'autres stablecoins/CBDC européens. |

```mermaid
graph TD
    subgraph BCE / Infrastructure Legacy
        A[Euro Numérique (DL3S/TARGET2)]
    end

    subgraph QUBITCOIN Network
        B[EuroDigitalBridge.sol] -- Mint/Burn --> A
        C[wEURd (ERC-20)]
        D[QBTCLiquidityPool.sol]
        E[CBDCRouter.sol]
        F[QBTC Token]
    end

    subgraph Utilisateurs / dApps
        G[dApp Institutionnelle]
    end

    A -- Settlement Agent --> B
    B -- Mint/Burn --> C
    C <--> D
    F <--> D
    D -- Price Feed --> E
    E -- Route Swaps --> D
    G -- Swap QBTC/EUR€ --> E
```

## 3. Composants Clés

### 3.1. EuroDigitalBridge.sol

Ce contrat assure la conversion 1:1 entre l'Euro Numérique et sa représentation on-chain, le **wEURd** (wrapped Euro digital).

-   **Conformité** : Intègre un registre d'identité (compatible ERC-3643) pour vérifier le statut KYC/AML de chaque participant. Les transactions sont limitées aux pays de l'UE/EEE.
-   **Sécurité** : Les opérations de `mint` et `burn` de grande valeur (> 100 000 €) exigent une signature post-quantique (FALCON-1024) de l'agent de règlement (banque/PSP agréé).
-   **Stabilité** : Un mécanisme de *cooldown* (délai d'attente) sur les retraits importants prévient les risques de *bank run*.
-   **Limites de détention** : Applique les limites recommandées par la BCE (ex: 3 000 € pour les particuliers).

### 3.2. QBTCLiquidityPool.sol

Cette pool AMM est le cœur économique de l'écosystème, conçue pour garantir une liquidité profonde et défendre la valeur du QBTC.

-   **Liquidité Concentrée** : Inspiré d'Uniswap V3, les fournisseurs de liquidité (LPs) peuvent allouer leur capital dans des fourchettes de prix spécifiques, maximisant l'efficacité.
-   **Trésorerie Stratégique (POL)** : La Fondation QUBITCOIN déploie une partie de ses réserves dans un mécanisme de **stabilisation progressive**. Le protocole utilise un rééquilibrage dynamique basé sur le TWAP pour maintenir une profondeur de liquidité optimale, combiné à une capacité d'**intervention discrétionnaire** en cas de conditions de marché exceptionnelles.
-   **Frais Dynamiques** : Les frais de transaction s'ajustent en fonction de la volatilité et de la taille de l'ordre, protégeant les LPs et décourageant la manipulation de marché.
-   **Oracle TWAP** : Fournit un prix moyen pondéré par le temps (TWAP) fiable, résistant à la manipulation, pour les autres protocoles DeFi.

### 3.3. CBDCRouter.sol

Ce contrat agit comme un agrégateur, offrant un point d'entrée unique pour toutes les opérations de swap impliquant des CBDC.

-   **Routage Intelligent** : Détermine automatiquement le chemin le plus efficace pour un swap (ex: QBTC → wEURd via la pool, ou wEURd → EUROC via un pont tiers).
-   **Interopérabilité** : Conçu selon les principes du WEF et de la BRI pour être compatible avec de futurs ponts vers d'autres CBDC (e-Krona, Digital Swiss Franc, etc.).

## 4. Conformité Réglementaire

| Réglementation | Implémentation dans le Dossier CBDC |
| :--- | :--- |
| **MiCA** | Le wEURd est un E-Money Token (EMT) émis par des agents de règlement agréés. La pool et le routeur sont des services de crypto-actifs (CASP) conformes. |
| **eIDAS 2.0** | Le registre d'identité (`IIdentityRegistry`) est conçu pour s'interfacer avec le portefeuille d'identité numérique européen (EUDI Wallet) pour un KYC/AML robuste. |
| **DORA** | L'utilisation de la cryptographie post-quantique et de contrats audités aide les institutions financières à respecter leurs obligations de résilience opérationnelle numérique. |

## 5. Conclusion

Ce dossier de compatibilité CBDC n'est pas seulement une fonctionnalité technique ; c'est une déclaration stratégique. Il ancre QUBITCOIN au cœur du futur système financier européen, en tant que partenaire technologique crédible, sécurisé et conforme pour les banques centrales, les institutions financières et les entreprises. Il garantit la pertinence et la valeur à long terme du token QBTC.


## 4. Rapport de Conformité

Un audit complet de la compatibilité de QUBITCOIN avec l'Euro Numérique a été réalisé. Le rapport confirme que l'architecture est **entièrement conforme** aux spécifications et principes de la BCE.

**➡️ [Consulter le Rapport de Conformité Euro Numérique](./compliance_report.md)**
