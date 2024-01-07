import algosdk, { waitForConfirmation, signTransaction } from "algosdk";

export const getAlgorandClients = (node) => {
  let ALGO_SERVER;
  let ALGO_INDEXER_SERVER;
  switch (node) {
    case "voi-testnet":
      ALGO_SERVER = "https://testnet-api.voi.nodly.io";
      ALGO_INDEXER_SERVER = "https://testnet-idx.voi.nodly.io";
      break;
    default:
    case "algorand-testnet":
      ALGO_SERVER = "https://testnet-api.algonode.cloud";
      ALGO_INDEXER_SERVER = "https://testnet-idx.algonode.cloud";
      break;
    case "algorand":
      ALGO_SERVER = "https://mainnet-api.algonode.cloud";
      ALGO_INDEXER_SERVER = "https://mainnet-idx.algonode.cloud";
      break;
  }
  const algodClient = new algosdk.Algodv2(
    process.env.ALGOD_TOKEN || "",
    process.env.ALGOD_SERVER || ALGO_SERVER,
    process.env.ALGOD_PORT || ""
  );
  const indexerClient = new algosdk.Indexer(
    process.env.INDEXER_TOKEN || "",
    process.env.INDEXER_SERVER || ALGO_INDEXER_SERVER,
    process.env.INDEXER_PORT || ""
  );
  return [algodClient, indexerClient];
};

export const signSendAndConfirm = async (algodClient, txns, sk) => {
  const stxns = txns.map((t) => signTransaction(t, sk));
  const txnIds = stxns.map(({ txID }) => txID);
  await algodClient.sendRawTransaction(stxns.map(({ blob }) => blob)).do();
  await Promise.all(
    stxns.map(({ txID }) => waitForConfirmation(algodClient, txID, 4))
  );
  return txnIds;
};
