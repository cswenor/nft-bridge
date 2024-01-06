import algosdk, {
  mnemonicToSecretKey,
  signTransaction,
  makePaymentTxnWithSuggestedParamsFromObject,
  waitForConfirmation,
} from "algosdk";
import dotenv from "dotenv";
import figlet from "figlet";
import { algoWalkAssetIds, alienTourismAssetIds } from "./wl.js";

dotenv.config();

console.log(figlet.textSync("NFT BRIDGE", { horizontalLayout: "full" }));

const { ADDR, MN, MN2, MN3, NODE, BRIDGE_ADDR } = process.env;

const bridgeAddress = BRIDGE_ADDR || "";

const addrTo = ADDR;

const mnemonic = MN || "";

const { addr: address, sk } = mnemonicToSecretKey(mnemonic);
const { addr: address2, sk: sk2 } = mnemonicToSecretKey(MN2 || "");
const { addr: address3, sk: sk3 } = mnemonicToSecretKey(MN3 || "");

const [node] = NODE.split(":");

console.log("address", address);
console.log("address2", address2);
console.log("address3", address3);
console.log("bridgeAddress", bridgeAddress);

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

const makeHelloTxn = async () => {
  return makePaymentTxnWithSuggestedParamsFromObject({
    from: address,
    to: bridgeAddress,
    amount: 0,
    note: new Uint8Array(Buffer.from("Hello world")),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const makeBridgeRequestTxn = async (assetId, addrTo) => {
  const transactionFee = 0.101;
  const serviceFee = 0.099;
  const totalFee = transactionFee + serviceFee;
  const req = {
    assetId,
    to: addrTo,
    amount: 1,
  };
  return makePaymentTxnWithSuggestedParamsFromObject({
    from: address,
    to: bridgeAddress,
    amount: Math.floor(totalFee * 1e6),
    note: new Uint8Array(Buffer.from(JSON.stringify(req))),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const signSendAndConfirm = async (txns, sk) => {
  const stxns = txns.map((t) => signTransaction(t, sk));
  console.log(stxns.map(({ txID }) => txID));
  await algodClient.sendRawTransaction(stxns.map(({ blob }) => blob)).do();
  await Promise.all(
    stxns.map(({ txID }) => waitForConfirmation(algodClient, txID, 4))
  );
};

const main = async () => {
  console.log("Do the thing!");
  // say hello world
  await signSendAndConfirm([await makeHelloTxn()], sk);
  // make request to bridge random asset
  await signSendAndConfirm(
    [
      await makeBridgeRequestTxn(
        Math.floor(Math.random() * 10_000_000_000),
        addrTo
      ),
    ],
    sk
  );
  // make request to bridge nfts from collections
  for (const assetIds of [algoWalkAssetIds, alienTourismAssetIds]) {
    await signSendAndConfirm(
      [
        await makeBridgeRequestTxn(
          assetIds[Math.floor(assetIds.length * Math.random())],
          addrTo
        ),
      ],
      sk
    );
  }
};

main();
