# DMChain 🧪

**DMChain** is an experimental Cosmos SDK-based blockchain network focused on extending core blockchain capabilities through custom modules. This chain is not production-grade, but serves as a proving ground for advanced module experimentation.

---

## 🧩 Available Modules

### 🔐 Multisig Module

The **Multisig Module** in DMChain allows secure, collaborative control over on-chain transactions via programmable multisig accounts. It goes beyond basic key-level multisig by supporting proposal-based workflows, signer management, and dynamic dispatching of transactions.

#### ✅ Core Features

- **Multisig Account Creation**  
  Initialize a multisig account by specifying participating signers and a signing threshold.

- **Signer Management**  
  Add or clean up signers from an existing multisig account, enabling dynamic group control.

- **Threshold Adjustment**  
  Modify the number of required signatures needed to approve and dispatch a proposal.

- **Proposal Initialization**  
  Create a proposal that wraps any on-chain transaction (e.g., token transfer, governance vote, custom module interaction).

- **Approval Workflow**  
  Individual signers approve proposals. Once the threshold is met, the proposal is eligible for dispatch.

- **One-click Approval + Execution**  
  If you're the last required signer, you can optionally approve and dispatch in a single action.

- **Proposal Cleanup**  
  Cancel or clean up expired, rejected, or executed proposals from the account’s proposal list.

---

## 🚀 How It Works

1. **Create a Multisig Account**  
   Define a group and a signing threshold.

2. **Submit a Proposal**  
   One of the signers proposes an on-chain action.

3. **Approve the Proposal**  
   Each signer submits their approval individually.

4. **Dispatch Once Approved**  
   Once enough signatures are collected, the transaction is executed on-chain.

5. **Manage Lifecycle**  
   Cancel or clean up proposals as needed.


## 👷‍♂️ Local Dev

```bash
# Clone the repo
git clone https://github.com/DaevMithran/dmchain
cd dmchain
```

- `make proto-gen` *Generates go code from proto files, stubs interfaces*

## Testnet

- `make testnet` *IBC testnet from chain <-> local cosmos-hub*
- `make sh-testnet` *Single node, no IBC. quick iteration*
- `local-ic chains` *See available testnets from the chains/ directory*
- `local-ic start <name>` *Starts a local chain with the given name*

## Local Images

- `make install`      *Builds the chain's binary*
- `make local-image`  *Builds the chain's docker image*

## Testing

- `go test ./... -v` *Unit test*
- `make ictest-*`  *E2E testing*
