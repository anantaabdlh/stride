package keeper

import (
	"fmt"

	epochstypes "github.com/Stride-Labs/stride/x/epochs/types"
	"github.com/Stride-Labs/stride/x/stakeibc/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
)

func (k Keeper) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	// every epoch
	k.Logger(ctx).Info(fmt.Sprintf("Handling epoch start %s %d", epochIdentifier, epochNumber))
	if epochIdentifier == "stride_epoch" {
		k.Logger(ctx).Info(fmt.Sprintf("Stride Epoch %d", epochNumber))
		depositInterval := int64(k.GetParam(ctx, types.KeyDepositInterval))
		if epochNumber%depositInterval == 0 {
			// TODO TEST-72 move this function to the keeper
			k.Logger(ctx).Info("Triggering deposits")
			depositRecords := k.GetAllDepositRecord(ctx)
			addr := k.accountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress().String()
			for _, depositRecord := range depositRecords {
				pstr := fmt.Sprintf("\tProcessing deposit {%d} {%s} {%d} {%s}", depositRecord.Id, depositRecord.Denom, depositRecord.Amount, depositRecord.Sender)
				k.Logger(ctx).Info(pstr)
				hostZone, hostZoneFound := k.GetHostZone(ctx, depositRecord.HostZoneId)
				if !hostZoneFound {
					k.Logger(ctx).Error("Host zone not found for deposit record {%d}", depositRecord.Id)
					continue
				}
				delegateAccount := hostZone.GetDelegationAccount()
				if delegateAccount == nil || delegateAccount.Address == "" {
					k.Logger(ctx).Error("Zone %s is missing a delegation address!", hostZone.ChainId)
					continue
				}
				delegateAddress := delegateAccount.Address
				// TODO(TEST-89): Set NewHeight relative to the most recent known gaia height (based on the LC)
				// TODO(TEST-90): why do we have two gaia LCs?
				timeoutHeight := clienttypes.NewHeight(0, 1000000000000)
				transferCoin := sdk.NewCoin(hostZone.GetIBCDenom(), sdk.NewInt(int64(depositRecord.Amount)))
				goCtx := sdk.WrapSDKContext(ctx)

				msg := ibctypes.NewMsgTransfer("transfer", hostZone.TransferChannelId, transferCoin, addr, delegateAddress, timeoutHeight, 0)
				_, err := k.transferKeeper.Transfer(goCtx, msg)
				if err != nil {
					pstr := fmt.Sprintf("\tERROR WITH DEPOSIT RECEIPT {%d}", depositRecord.Id)
					k.Logger(ctx).Info(pstr)
					panic(err)
				} else {
					// TODO TEST-71 what should we do if this transfer fails
					k.RemoveDepositRecord(ctx, depositRecord.Id)
				}
			}
		}

		// DELEGATE FROM DELEGATION ACCOUNT
		delegateInterval := int64(k.GetParam(ctx, types.KeyDelegateInterval))
		if epochNumber%delegateInterval == 0 {
			// get Gaia LC height
			k.ProcessDelegationStaking(ctx)
		}

		//TODO(TEST-112) make sure to update host LCs here!
		exchangeRateInterval := int64(k.GetParam(ctx, types.KeyExchangeRateInterval))
		if epochNumber%exchangeRateInterval == 0 { // allow a few blocks from UpdateUndelegatedBal to avoid conflicts
			// GET LATEST HEIGHT
			// TODO(NOW) wrap this into a function
			var latestHeightGaia int64 // defaults to 0
			// get light client's latest height
			connectionID := "connection-0"
			conn, found := k.IBCKeeper.ConnectionKeeper.GetConnection(ctx, connectionID)
			if !found {
				k.Logger(ctx).Info(fmt.Sprintf("invalid connection id, \"%s\" not found", connectionID))
			}
			clientState, found := k.IBCKeeper.ClientKeeper.GetClientState(ctx, conn.ClientId)
			if !found {
				k.Logger(ctx).Info(fmt.Sprintf("client id \"%s\" not found for connection \"%s\"", conn.ClientId, connectionID))
				// latestHeightGaia = 0
			} else {
				// TODO(TEST-119) get stAsset supply at SAME time as gaia height
				// TODO(TEST-112) check on safety of castng uint64 to int64
				latestHeightGaia = int64(clientState.GetLatestHeight().GetRevisionHeight())
				// set query height var in store for access within callbacks (to avoid issues with passing in height by value)
				// TODO(now) cleanup

				// TODO(TEST-97) update only when balances, delegatedBalances and stAsset supply are results from the same block
				k.ProcessUpdateBalances(ctx, latestHeightGaia)
			}
		}

		// if epochNumber%exchangeRateInterval == 4 { // allow a few blocks from UpdateUndelegatedBal to avoid conflicts
		// 	// TODO(TEST-97) update only when balances, delegatedBalances and stAsset supply are results from the same block
		// }
		// if epochNumber%exchangeRateInterval == 8 && (epochNumber > 100) { // allow a few blocks from UpdateDelegatedBal to avoid conflicts & wait until chain has registered zones to calc exch rate
		// 	// TODO(TEST-97) update only when balances, delegatedBalances and stAsset supply are results from the same block
		// 	k.ProcessUpdateExchangeRate(ctx)
		// }

	}
}

func (k Keeper) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	// every epoch
	k.Logger(ctx).Info(fmt.Sprintf("Handling epoch end %s %d", epochIdentifier, epochNumber))

}

// Hooks wrapper struct for incentives keeper
type Hooks struct {
	k Keeper
}

var _ epochstypes.EpochHooks = Hooks{}

func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// epochs hooks
func (h Hooks) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	h.k.BeforeEpochStart(ctx, epochIdentifier, epochNumber)
}

func (h Hooks) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	h.k.AfterEpochEnd(ctx, epochIdentifier, epochNumber)
}
